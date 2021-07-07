package apply

import (
	"context"
	"fmt"
	"log"

	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Find existing object or create if if it doesn't exists
func findOrCreateObject(ctx context.Context, client k8sclient.Client, obj *uns.Unstructured) (*uns.Unstructured, string, error) {
	name := obj.GetName()
	namespace := obj.GetNamespace()
	if name == "" {
		return nil, "", errors.Errorf("Object %s has no name", obj.GroupVersionKind().String())
	}
	gvk := obj.GroupVersionKind()
	// used for logging and errors
	objDesc := fmt.Sprintf("(%s) %s/%s", gvk.String(), namespace, name)
	log.Printf("reconciling %s", objDesc)

	if err := IsObjectSupported(obj); err != nil {
		return nil, objDesc, errors.Wrapf(err, "object %s unsupported", objDesc)
	}

	// Get existing
	existing := &uns.Unstructured{}
	existing.SetGroupVersionKind(gvk)
	err := client.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, existing)

	if err != nil && apierrors.IsNotFound(err) {
		log.Printf("does not exist, creating %s", objDesc)
		err := client.Create(ctx, obj)
		if err != nil {
			return nil, objDesc, errors.Wrapf(err, "could not create %s", objDesc)
		}
		log.Printf("successfully created %s", objDesc)
		return obj, objDesc, nil
	}

	return existing, objDesc, err
}

// ApplyObject applies the desired object against the apiserver,
// merging it with any existing objects if already present.
func ApplyObject(ctx context.Context, client k8sclient.Client, obj *uns.Unstructured) error {

	existing, objDesc, err := findOrCreateObject(ctx, client, obj)

	if existing == nil {
		return nil
	}

	if err != nil {
		return errors.Wrapf(err, "could not retrieve existing %s", objDesc)
	}

	// Merge the desired object with what actually exists
	if err := MergeObjectForUpdate(existing, obj); err != nil {
		return errors.Wrapf(err, "could not merge object %s with existing", objDesc)
	}
	if !equality.Semantic.DeepEqual(existing, obj) {
		if err := client.Update(ctx, obj); err != nil {
			return errors.Wrapf(err, "could not update object %s", objDesc)
		} else {
			log.Printf("update was successful")
		}
	}

	return nil
}

// ApplyObjects it applies a list of desired objects after merging them.
func ApplyObjects(ctx context.Context, client k8sclient.Client, objs []*uns.Unstructured) error {

	var lastObj, existing *uns.Unstructured = nil, nil
	var objDesc string = ""
	var err error = nil

	existing, objDesc, err = findOrCreateObject(ctx, client, objs[0])
	if existing == nil {
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "could not retrieve existing %s", objDesc)
	}

	for _, obj := range objs {
		if lastObj != nil {
			// Merge the desired object with what actually exists
			if err := MergeObjectForUpdate(existing, obj); err != nil {
				return errors.Wrapf(err, "could not merge object %s with existing", objDesc)
			}
		}
		lastObj = obj
	}

	if !equality.Semantic.DeepEqual(existing, lastObj) && lastObj != nil {
		if err := client.Update(ctx, lastObj); err != nil {
			return errors.Wrapf(err, "could not update object %s", objDesc)
		} else {
			log.Printf("update was successful")
		}
	}

	return nil
}
