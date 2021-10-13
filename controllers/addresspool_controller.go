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
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metallbv1alpha1 "github.com/metallb/metallb-operator/api/v1alpha1"
	"github.com/metallb/metallb-operator/pkg/render"
)

// AddressPoolReconciler reconciles a AddressPool object
type AddressPoolReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Namespace string
}

const (
	RetryPeriod = 1 * time.Minute
)

// +kubebuilder:rbac:groups=metallb.io,resources=addresspools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metallb.io,resources=addresspools/status,verbs=get;update;patch

func (r *AddressPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Starting AddressPool reconcile loop for %v", req.NamespacedName))
	defer r.Log.Info(fmt.Sprintf("Finish AddressPool reconcile loop for %v", req.NamespacedName))

	err := reconcileConfigMap(ctx, r.Client, r.Log, r.Namespace)
	if errors.As(err, &render.RenderingFailed{}) {
		r.Log.Error(err, "configmap rendering failed", "controller", "addresspool")
		return ctrl.Result{}, nil
	}
	if err != nil {
		r.Log.Error(err, "failed to reconcile configmap", "controller", "addresspool")
		return ctrl.Result{RequeueAfter: RetryPeriod}, err
	}
	return ctrl.Result{}, nil
}

func (r *AddressPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&metallbv1alpha1.AddressPool{}).
		Complete(r)
}
