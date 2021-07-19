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
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/pkg/apply"
	"github.com/metallb/metallb-operator/pkg/platform"
	"github.com/metallb/metallb-operator/pkg/render"
	"github.com/metallb/metallb-operator/pkg/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const defaultMetalLBCrName = "metallb"

// MetalLBReconciler reconciles a MetalLB object
type MetalLBReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	PlatformInfo platform.PlatformInfo
	Namespace    string
}

var ManifestPath = "./bindata/deployment"

// Namespace Scoped
// +kubebuilder:rbac:groups=apps,namespace=metallb-system,resources=deployments;daemonsets,verbs=get;list;watch;create;update;patch;delete

// Cluster Scoped
// +kubebuilder:rbac:groups=metallb.io,resources=metallbs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metallb.io,resources=metallbs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=policy,resources=podsecuritypolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metallb.io,resources=metallbs/finalizers,verbs=delete;get;update;patch

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
		}
		return ctrl.Result{}, nil // Return success to avoid requeue
	}

	result, condition, err := r.reconcileResource(ctx, req, instance)
	if condition != "" {
		errorMsg, wrappedErrMsg := "", ""
		if err != nil {
			if errors.Unwrap(err) != nil {
				wrappedErrMsg = errors.Unwrap(err).Error()
			}
		}
		if err := status.Update(context.TODO(), r.Client, instance, condition, errorMsg, wrappedErrMsg); err != nil {
			logger.Info("Failed to update metallb status", "Desired status", status.ConditionAvailable)
		}
	}
	return result, err
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
	return ctrl.NewControllerManagedBy(mgr).
		For(&metallbv1beta1.MetalLB{}).
		Complete(r)
}

func (r *MetalLBReconciler) syncMetalLBResources(config *metallbv1beta1.MetalLB) error {
	logger := r.Log.WithName("syncMetalLBResources")
	logger.Info("Start")
	data := render.MakeRenderData()

	data.Data["SpeakerImage"] = os.Getenv("SPEAKER_IMAGE")
	data.Data["ControllerImage"] = os.Getenv("CONTROLLER_IMAGE")
	data.Data["IsOpenShift"] = r.PlatformInfo.IsOpenShift()
	data.Data["NameSpace"] = r.Namespace
	objs, err := render.RenderDir(ManifestPath, &data)
	if err != nil {
		logger.Error(err, "Fail to render config daemon manifests")
		return err
	}

	for _, obj := range objs {
		if err := controllerutil.SetControllerReference(config, obj, r.Scheme); err != nil {
			return errors.Wrapf(err, "Failed to set controller reference to %s %s", obj.GetNamespace(), obj.GetName())
		}
		if err := apply.ApplyObject(context.TODO(), r.Client, obj); err != nil {
			return errors.Wrapf(err, "could not apply (%s) %s/%s", obj.GroupVersionKind(), obj.GetNamespace(), obj.GetName())
		}
	}
	return nil
}
