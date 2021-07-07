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
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	log := r.Log.WithValues("addresspool", req.NamespacedName)
	log.Info("Reconciling AddressPool resource")

	instance := &metallbv1alpha1.AddressPool{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	err := r.syncMetalLBAddressPool(instance)
	if err != nil {
		errors.Wrap(err, "Failed to create address-pool config map")
		return ctrl.Result{RequeueAfter: RetryPeriod}, err
	}

	log.Info("Reconcile complete")
	return ctrl.Result{}, nil
}

func (r *AddressPoolReconciler) syncMetalLBAddressPool(instance *metallbv1alpha1.AddressPool) error {
	data := render.MakeRenderData()
	data.Data["Name"] = instance.Spec.Name
	data.Data["Protocol"] = instance.Spec.Protocol
	data.Data["AutoAssign"] = *instance.Spec.AutoAssign
	data.Data["Addresses"] = instance.Spec.Addresses
	objs, err := render.RenderDir(AddressPoolManifestPath, &data)
	if err != nil {
		return errors.Wrapf(err, "Fail to render address-pool manifest")
	}

	for _, obj := range objs {
		if err := controllerutil.SetControllerReference(instance, obj, r.Scheme); err != nil {
			return errors.Wrapf(err, "Failed to set controller reference to %s %s",
				obj.GetNamespace(), obj.GetName())
		}
		if err := apply.ApplyObject(context.Background(), r.Client, obj); err != nil {
			err = errors.Wrapf(err, "could not apply (%s) %s/%s", obj.GroupVersionKind(),
				obj.GetNamespace(), obj.GetName())
		}
	}

	return err
}

func (r *AddressPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&metallbv1alpha1.AddressPool{}).
		Complete(r)
}
