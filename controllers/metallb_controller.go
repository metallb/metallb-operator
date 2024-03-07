/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/pkg/apply"
	"github.com/metallb/metallb-operator/pkg/helm"
	"github.com/metallb/metallb-operator/pkg/params"
	"github.com/metallb/metallb-operator/pkg/platform"
	"github.com/metallb/metallb-operator/pkg/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	defaultMetalLBCrName       = "metallb"
	MetalLBChartPathController = "./bindata/deployment/helm/metallb"
	FRRK8SChartPathController  = "./bindata/deployment/helm/frr-k8s"
)

// MetalLBReconciler reconciles a MetalLB object
type MetalLBReconciler struct {
	client.Client
	metalLBChart *helm.MetalLBChart
	frrk8sChart  *helm.FRRK8SChart
	Log          logr.Logger
	Scheme       *runtime.Scheme
	PlatformInfo platform.PlatformInfo
	Namespace    string
	EnvConfig    params.EnvConfig
}

var MetalLBChartPath = MetalLBChartPathController
var FRRK8SChartPath = FRRK8SChartPathController

// Namespace Scoped
// +kubebuilder:rbac:groups=apps,namespace=metallb-system,resources=deployments;daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=podmonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",namespace=metallb-system,resources=services,verbs=create;delete;get;update;patch
// +kubebuilder:rbac:groups="coordination.k8s.io",namespace=metallb-system,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",namespace=metallb-system,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",namespace=metallb-system,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Cluster Scoped
// +kubebuilder:rbac:groups=metallb.io,resources=metallbs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metallb.io,resources=metallbs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=policy,resources=podsecuritypolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metallb.io,resources=metallbs/finalizers,verbs=delete;get;update;patch
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=create;delete;get;update;patch;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=create;delete;get;update;patch;list;watch

func (r *MetalLBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log.WithValues("metallb", req.NamespacedName)

	instance := &metallbv1beta1.MetalLB{}
	err := r.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	if req.Name != defaultMetalLBCrName {
		err := fmt.Errorf("MetalLB resource name must be '%s'", defaultMetalLBCrName)
		logger.Error(err, "Invalid MetalLB resource name", "name", req.Name)
		if err := status.Update(context.TODO(), r.Client, instance, status.ConditionDegraded, "IncorrectMetalLBResourceName", fmt.Sprintf("Incorrect MetalLB resource name: %s", req.Name)); err != nil {
			logger.Error(err, "Failed to update metallb status", "Desired status", status.ConditionDegraded)
			return ctrl.Result{}, nil // Return success to avoid requeue
		}
		logger.Info("updated metallb status successfully", "condition", status.ConditionDegraded, "resource name", req.Name)
		return ctrl.Result{}, nil // Return success to avoid requeue
	}

	result, condition, err := r.reconcileResource(ctx, req, instance)
	if condition != "" {
		errorMsg, wrappedErrMsg := condition, ""
		if err != nil {
			errorMsg = "internal error"
			if errors.Unwrap(err) != nil {
				wrappedErrMsg = errors.Unwrap(err).Error()
			}
		}
		if err := status.Update(context.TODO(), r.Client, instance, condition, errorMsg, wrappedErrMsg); err != nil {
			logger.Error(err, "Failed to update metallb status", "Desired status", condition)
			return ctrl.Result{}, err
		}
		logger.Info("updated metallb status successfully", "condition", condition, "resource name", req.Name)
	}
	return result, nil
}

func (r *MetalLBReconciler) reconcileResource(ctx context.Context, req ctrl.Request, instance *metallbv1beta1.MetalLB) (ctrl.Result, string, error) {
	err := r.syncMetalLBResources(instance)
	if err != nil {
		return ctrl.Result{}, status.ConditionDegraded, errors.Wrapf(err, "FailedToSyncMetalLBResources")
	}
	err = status.IsMetalLBAvailable(context.TODO(), r.Client, req.NamespacedName.Namespace)
	if err != nil {
		if _, ok := err.(status.MetalLBResourcesNotReadyError); ok {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, status.ConditionProgressing, nil
		}
		return ctrl.Result{}, status.ConditionProgressing, err
	}
	return ctrl.Result{}, status.ConditionAvailable, nil
}

func (r *MetalLBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var err error
	r.metalLBChart, err = helm.NewMetalLBChart(MetalLBChartPath, defaultMetalLBCrName, r.Namespace, r.Client)
	if err != nil {
		return err
	}
	r.frrk8sChart, err = helm.NewFRRK8SChart(FRRK8SChartPath, "frr-k8s", r.Namespace)
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&metallbv1beta1.MetalLB{}).
		Complete(r)
}

func (r *MetalLBReconciler) syncMetalLBResources(config *metallbv1beta1.MetalLB) error {
	logger := r.Log.WithName("syncMetalLBResources")
	logger.Info("Start")

	err := validateBGPMode(config, r.EnvConfig.IsOpenshift)
	if err != nil {
		return err
	}
	objs := []*unstructured.Unstructured{}
	toDel := []*unstructured.Unstructured{}
	frrk8sObjs, err := r.frrk8sChart.Objects(r.EnvConfig, config)
	if err != nil {
		return err
	}

	if config.BGPBackend() == params.FRRK8sMode {
		objs = append(objs, frrk8sObjs...)
	} else {
		toDel = append(toDel, frrk8sObjs...)
	}

	mlbObjs, err := r.metalLBChart.Objects(r.EnvConfig, config)
	if err != nil {
		return err
	}
	objs = append(objs, mlbObjs...)

	for _, obj := range toDel {
		err := r.Client.Delete(context.Background(), obj)
		if err != nil && !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "could not delete (%s) %s/%s", obj.GroupVersionKind(), obj.GetNamespace(), obj.GetName())
		}
	}

	for _, obj := range objs {
		objKind := obj.GetKind()
		// Skip applying role and role binding object, because with the operator these are being set outside,
		// either in manifests or via the csv.
		if objKind == "Role" || objKind == "RoleBinding" {
			continue
		}
		objNS := obj.GetNamespace()
		if objNS != "" { // Avoid setting reference on a cluster-scoped resource.
			if err := controllerutil.SetControllerReference(config, obj, r.Scheme); err != nil {
				return errors.Wrapf(err, "Failed to set controller reference to %s %s", objNS, obj.GetName())
			}
		}
		if err := apply.ApplyObject(context.TODO(), r.Client, obj); err != nil {
			return errors.Wrapf(err, "could not apply (%s) %s/%s", obj.GroupVersionKind(), objNS, obj.GetName())
		}
	}

	return nil
}

func validateBGPMode(config *metallbv1beta1.MetalLB, isOpenshift bool) error {
	if config.Spec.BGPBackend != "" &&
		config.Spec.BGPBackend != params.NativeMode &&
		config.Spec.BGPBackend != params.FRRK8sMode &&
		config.Spec.BGPBackend != params.FRRMode {
		return fmt.Errorf("unsupported bgp backend %s", config.Spec.BGPBackend)
	}
	if isOpenshift && config.Spec.BGPBackend == params.NativeMode {
		return fmt.Errorf("native mode is not supported on openshift")
	}
	return nil
}
