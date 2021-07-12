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
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"

	metallbv1alpha1 "github.com/metallb/metallb-operator/api/v1alpha1"
	"github.com/metallb/metallb-operator/pkg/apply"
	"github.com/metallb/metallb-operator/pkg/render"
)

// AddressPoolReconciler reconciles a AddressPool object
type AddressPoolReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

const (
	AddressPoolManifestPath = "./bindata/configuration/address-pool"
	RetryPeriod             = 5 * time.Minute
)

// +kubebuilder:rbac:groups=metallb.io,resources=addresspools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metallb.io,resources=addresspools/status,verbs=get;update;patch

func (r *AddressPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Starting AddressPool reconcile loop for %v", req.NamespacedName))

	instance := &metallbv1alpha1.AddressPool{}
	defer r.Log.Info(fmt.Sprintf("Finish AddressPool reconcile loop for %v", req.NamespacedName))

	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			err = r.syncMetallbAddressPools(req)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	err := r.syncMetalLBAddressPool(instance)
	if err != nil {
		r.Log.Info(fmt.Sprintf("sync MetalLB addresspool failed %s", err))
		return ctrl.Result{RequeueAfter: RetryPeriod}, err
	}

	return ctrl.Result{}, nil
}

func renderObject(instance *metallbv1alpha1.AddressPool) ([]*unstructured.Unstructured, error) {
	data := render.MakeRenderData()
	data.Data["Name"] = instance.Spec.Name
	data.Data["Protocol"] = instance.Spec.Protocol
	data.Data["AutoAssign"] = *instance.Spec.AutoAssign
	data.Data["Addresses"] = instance.Spec.Addresses
	objs, err := render.RenderDir(AddressPoolManifestPath, &data)
	if err != nil || objs == nil {
		return nil, fmt.Errorf("Fail to render address-pool manifest err %v objs %v", err, objs)
	}

	if len(objs) > 1 {
		return nil, fmt.Errorf("Fail to render we are expecting only one object and get %d", len(objs))
	}

	return objs, err
}

func (r *AddressPoolReconciler) syncMetalLBAddressPool(instance *metallbv1alpha1.AddressPool) error {
	objs, err := renderObject(instance)

	if err != nil {
		return fmt.Errorf("Fail to render address-pool manifest %v", err)
	}

	for _, obj := range objs {
		if err := apply.ApplyObject(context.Background(), r.Client, obj); err != nil {
			err = fmt.Errorf("could not apply (%s) %s/%s err %v", obj.GroupVersionKind(),
				obj.GetNamespace(), obj.GetName(), err)
		}
	}

	return err
}

func (r *AddressPoolReconciler) syncMetallbAddressPools(req ctrl.Request) error {
	instanceList := &metallbv1alpha1.AddressPoolList{}
	objs := make([]*unstructured.Unstructured, 0)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: req.Namespace,
		},
	}

	// Delete the exiting configMap
	if err := r.Delete(context.Background(), configMap); err != nil {
		// if we don't have ConfigMap then there is nothing to do
		if errors.IsNotFound(err) {
			return nil
		}
		r.Log.Info(fmt.Sprintf("Failed to delete existing Configmap %s", err))
		return err
	}

	if err := r.List(context.Background(), instanceList); err != nil {
		r.Log.Info(fmt.Sprintf("Failed to get existing addresspool objects %s", err))
		return err
	}

	for _, instance := range instanceList.Items {
		objslist, err := renderObject(&instance)
		if err != nil {
			return fmt.Errorf("Failed to render address-pool manifest %v", err)
		}

		for _, obj := range objslist {
			objs = append(objs, obj)
		}
	}

	if len(objs) > 0 {
		if err := apply.ApplyObjects(context.Background(), r.Client, objs); err != nil {
			return fmt.Errorf("Failed to ApplyObjects %v", err)
		}
	}

	return nil
}

func (r *AddressPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&metallbv1alpha1.AddressPool{}).
		Complete(r)
}
