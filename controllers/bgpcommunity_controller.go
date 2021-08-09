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
	"github.com/metallb/metallb-operator/pkg/apply"
	"github.com/metallb/metallb-operator/pkg/render"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metallbv1alpha1 "github.com/metallb/metallb-operator/api/v1alpha1"
)

// BGPCommunityReconciler reconciles a BGPCommunity object
type BGPCommunityReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Namespace string
}

const MetalLBConfigMapName = "config"

var BGPCommunityManifestPath = "./bindata/configuration/bgp-community"

//+kubebuilder:rbac:groups=metallb.io,resources=bgpcommunities,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=metallb.io,resources=bgpcommunities/status,verbs=get;update;patch

func (r *BGPCommunityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Starting BGP Community reconcile loop for %v", req.NamespacedName))

	instance := &metallbv1alpha1.BGPCommunity{}
	defer r.Log.Info(fmt.Sprintf("Finish BGP Community reconcile loop for %v", req.NamespacedName))

	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			err = r.syncMetalLBBGPCommunities(req)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	err := r.syncMetalLBBGPCommunity(instance)
	if err != nil {
		r.Log.Info(fmt.Sprintf("sync MetalLB BGP Community failed %s", err))
		return ctrl.Result{RequeueAfter: RetryPeriod}, err
	}

	return ctrl.Result{}, nil
}

func (r *BGPCommunityReconciler) renderObject(instance *metallbv1alpha1.BGPCommunity) ([]*unstructured.Unstructured, error) {
	data := render.MakeRenderData()
	data.Data["BGPCommunity"] = instance.Spec.BGPCommunity
	data.Data["NameSpace"] = r.Namespace
	data.Data["ConfigMapName"] = MetalLBConfigMapName
	objs, err := render.RenderDir(BGPCommunityManifestPath, &data)
	if err != nil {
		return nil, fmt.Errorf("Fail to render BGPCommunity manifest err %v", err)
	}

	if len(objs) > 1 {
		return nil, fmt.Errorf("Fail to render we are expecting only one object and get %d", len(objs))
	}

	return objs, err
}

func (r *BGPCommunityReconciler) syncMetalLBBGPCommunity(instance *metallbv1alpha1.BGPCommunity) error {
	objs, err := r.renderObject(instance)

	if err != nil {
		return fmt.Errorf("Fail to render bgp community manifest %v", err)
	}

	for _, obj := range objs {
		if err := apply.ApplyObject(context.Background(), r.Client, obj); err != nil {
			return fmt.Errorf("could not apply (%s) %s/%s err %v", obj.GroupVersionKind(),
				obj.GetNamespace(), obj.GetName(), err)
		}
	}

	return err
}

func (r *BGPCommunityReconciler) syncMetalLBBGPCommunities(req ctrl.Request) error {
	instanceList := &metallbv1alpha1.BGPCommunityList{}
	objs := make([]*unstructured.Unstructured, 0)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MetalLBConfigMapName,
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
		r.Log.Info(fmt.Sprintf("Failed to get existing peer objects %s", err))
		return err
	}

	for _, instance := range instanceList.Items {
		objslist, err := r.renderObject(&instance)
		if err != nil {
			return fmt.Errorf("Failed to render peer manifest %v", err)
		}

		objs = append(objs, objslist...)
	}

	if len(objs) > 0 {
		if err := apply.ApplyObjects(context.Background(), r.Client, objs); err != nil {
			return fmt.Errorf("Failed to ApplyObjects %v", err)
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BGPCommunityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&metallbv1alpha1.BGPCommunity{}).
		Complete(r)
}
