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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	metallbv1alpha1 "github.com/metallb/metallb-operator/api/v1alpha1"
	"github.com/metallb/metallb-operator/pkg/render"
)

// BGPPeer Reconciler reconciles a Peer object
type BGPPeerReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Namespace string
}

//+kubebuilder:rbac:groups=metallb.io,resources=bgppeers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=metallb.io,resources=bgppeers/status,verbs=get;update;patch

func (r *BGPPeerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Starting BGP Peer reconcile loop for %v", req.NamespacedName))
	defer r.Log.Info(fmt.Sprintf("Finish BGP Peer reconcile loop for %v", req.NamespacedName))

	err := reconcileConfigMap(ctx, r.Client, r.Log, r.Namespace)
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

func (r *BGPPeerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&metallbv1alpha1.BGPPeer{}).
		Complete(r)
}
