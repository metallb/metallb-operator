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
	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/pkg/render"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const RetryPeriod = time.Minute

// ConfigMap Reconciler reconciles a Peer object
type ConfigMapReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Namespace string
}

//+kubebuilder:rbac:groups=metallb.io,resources=bgppeers,verbs=get;list;watch;
//+kubebuilder:rbac:groups=metallb.io,resources=bgppeers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=metallb.io,resources=addresspools,verbs=get;list;watch;create;
//+kubebuilder:rbac:groups=metallb.io,resources=bfdprofiles,verbs=get;list;watch;create;
//+kubebuilder:rbac:groups=metallb.io,resources=bfdprofiles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=metallb.io,resources=addresspools/status,verbs=get;update;patch

func (r *ConfigMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Starting ConfigMap reconcile loop for %v", req.NamespacedName))
	defer r.Log.Info(fmt.Sprintf("Finish ConfigMap reconcile loop for %v", req.NamespacedName))

	err := reconcileConfigMap(ctx, r.Client, r.Log, r.Namespace, r.Scheme)
	if errors.As(err, &render.RenderingFailed{}) {
		r.Log.Error(err, "configmap rendering failed", "controller", "bgppeer")
		return ctrl.Result{}, nil
	}
	if err != nil {
		r.Log.Error(err, "failed to reconcile configmap", "controller", "bgppeer")
		return ctrl.Result{RequeueAfter: RetryPeriod}, err
	}
	return ctrl.Result{}, nil
}

func (r *ConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	cmPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return false
		}
		return cm.Name == ConfigMapName
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}, builder.WithPredicates(cmPredicate)).
		Watches(&source.Kind{Type: &metallbv1beta1.BGPPeer{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &metallbv1beta1.AddressPool{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &metallbv1beta1.BFDProfile{}}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
