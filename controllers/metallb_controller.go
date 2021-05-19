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

	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metallbv1alpha1 "github.com/fedepaol/metallboperator/api/v1alpha1"
	"github.com/fedepaol/metallboperator/pkg/apply"
	"github.com/fedepaol/metallboperator/pkg/render"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// MetallbReconciler reconciles a Metallb object
type MetallbReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

var ManifestPath = "./bindata"

// +kubebuilder:rbac:groups=metallb.quay.io/fpaoline/metallboperator,resources=metallbs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metallb.quay.io/fpaoline/metallboperator,resources=metallbs/status,verbs=get;update;patch

func (r *MetallbReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("metallb", req.NamespacedName)

	instance := &metallbv1alpha1.Metallb{}
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

	err = r.syncMetalLBResources(instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MetallbReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&metallbv1alpha1.Metallb{}).
		Complete(r)
}

func (r *MetallbReconciler) syncMetalLBResources(config *metallbv1alpha1.Metallb) error {
	logger := r.Log.WithName("syncMetalLBResources")
	logger.Info("Start")
	// var err error
	objs := []*uns.Unstructured{}
	data := render.MakeRenderData()

	// data.Data["Image"] = os.Getenv("METALLB_IMAGE") // TODO Make images parametric here
	objs, err := render.RenderDir(ManifestPath, &data)
	if err != nil {
		logger.Error(err, "Fail to render config daemon manifests")
		return err
	}

	for _, obj := range objs {
		// Mark the object to be GC'd if the owner is deleted.
		if err := controllerutil.SetControllerReference(config, obj, r.Scheme); err != nil {
			return errors.Wrapf(err, "Failed to set controller reference to %s %s", obj.GetNamespace(), obj.GetName())
		}

		// Open question: should an error here indicate we will never retry?
		if err := apply.ApplyObject(context.TODO(), r.Client, obj); err != nil {
			err = errors.Wrapf(err, "could not apply (%s) %s/%s", obj.GroupVersionKind(), obj.GetNamespace(), obj.GetName())
		}
	}

	return nil
}
