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

	"k8s.io/apimachinery/pkg/api/errors"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	metallbv1alpha1 "github.com/metallb/metallb-operator/api/v1alpha1"
	"github.com/metallb/metallb-operator/pkg/apply"
	"github.com/metallb/metallb-operator/pkg/render"
)

// BGPPeer Reconciler reconciles a Peer object
type BGPPeerReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Namespace string
}

const BGPPeerManifestPathContoller = "./bindata/configuration/bgp-peer"

var BGPPeerManifestPath = BGPPeerManifestPathContoller

//+kubebuilder:rbac:groups=metallb.io,resources=bgppeers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=metallb.io,resources=bgppeers/status,verbs=get;update;patch

func (r *BGPPeerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Starting BGP Peer reconcile loop for %v", req.NamespacedName))

	instance := &metallbv1alpha1.BGPPeer{}
	defer r.Log.Info(fmt.Sprintf("Finish BGP Peer reconcile loop for %v", req.NamespacedName))

	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			err = r.syncMetalLBBGPPeers()
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	err := r.syncMetalLBBGPPeer(instance)
	if err != nil {
		r.Log.Info(fmt.Sprintf("sync MetalLB BGP Peer failed %s", err))
		return ctrl.Result{RequeueAfter: RetryPeriod}, err
	}

	return ctrl.Result{}, nil
}

func (r *BGPPeerReconciler) renderObject(instance *metallbv1alpha1.BGPPeer) ([]*uns.Unstructured, error) {
	data := render.MakeRenderData()
	data.Data["Address"] = instance.Spec.Address
	data.Data["ASN"] = instance.Spec.ASN
	data.Data["MyASN"] = instance.Spec.MyASN
	data.Data["Port"] = instance.Spec.Port
	data.Data["HoldTime"] = instance.Spec.HoldTime
	data.Data["RouterID"] = instance.Spec.RouterID
	data.Data["Password"] = instance.Spec.Password
	data.Data["NodeSelectors"] = instance.Spec.NodeSelectors
	data.Data["NameSpace"] = r.Namespace
	data.Data["ConfigMapName"] = apply.MetalLBConfigMap
	data.Data["SrcAddress"] = instance.Spec.SrcAddress

	objs, err := render.RenderDir(BGPPeerManifestPath, &data)
	if err != nil {
		return nil, fmt.Errorf("Fail to render bgp peer manifest err %v", err)
	}

	if len(objs) > 1 {
		return nil, fmt.Errorf("Fail to render we are expecting only one object and get %d", len(objs))
	}

	return objs, err
}

func (r *BGPPeerReconciler) syncMetalLBBGPPeer(instance *metallbv1alpha1.BGPPeer) error {
	objs, err := r.renderObject(instance)

	if err != nil {
		return fmt.Errorf("Fail to render bgp-peer manifest %v", err)
	}

	for _, obj := range objs {
		if err := apply.ApplyObject(context.Background(), r.Client, obj); err != nil {
			return fmt.Errorf("could not apply (%s) %s/%s err %v", obj.GroupVersionKind(),
				obj.GetNamespace(), obj.GetName(), err)
		}
	}

	return err
}

func (r *BGPPeerReconciler) syncMetalLBBGPPeers() error {
	instanceList := &metallbv1alpha1.BGPPeerList{}
	specList := make([]metallbv1alpha1.BGPPeerSpec, 0)

	if err := r.List(context.Background(), instanceList); err != nil {
		r.Log.Info(fmt.Sprintf("Failed to get existing peer objects %s", err))
		return err
	}
	for _, obj := range instanceList.Items {
		specList = append(specList, obj.Spec)
	}

	if err := apply.UpdateConfigMapObjs(context.Background(), r.Client,
		func(m *apply.ConfigMapData) {
			m.Peers = specList
		}, r.Namespace); err != nil {
		return fmt.Errorf("Failed to update ConfigMap %s", err)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BGPPeerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&metallbv1alpha1.BGPPeer{}).
		Complete(r)
}
