package apply

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"log"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func UpdateConfigMapObjs(ctx context.Context, client k8sclient.Client, configMapModifier func(m *ConfigMapData), namespace string) error {
	cur := ConfigMapData{}
	var configMap corev1.ConfigMap

	if err := client.Get(ctx, k8sclient.ObjectKey{Name: MetalLBConfigMap, Namespace: namespace}, &configMap); err != nil {
		return nil
	}
	if err := yaml.Unmarshal([]byte(configMap.Data[MetalLBConfigMap]), &cur); err != nil {
		return err
	}
	configMapModifier(&cur)
	resData, err := yaml.Marshal(cur)
	if err != nil {
		return err
	}
	configMap.Data[MetalLBConfigMap] = string(resData)
	// If there is no more any objects then its safe to delete the configmap
	if len(cur.AddressPools) == 0 && len(cur.Peers) == 0 {
		if err := client.Delete(ctx, &configMap); err != nil {
			return fmt.Errorf("Failed to delete configmap err %s", err)
		}
	} else {
		if err := client.Update(ctx, &configMap); err != nil {
			return fmt.Errorf("could not update configmap err %s", err)
		}
	}
	return nil
}

func getExistingObject(ctx context.Context, client k8sclient.Client, obj *uns.Unstructured) (*uns.Unstructured, string, error) {
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
	return existing, objDesc, err
}

// ApplyObject applies the desired object against the apiserver,
// merging it with any existing objects if already present.
func ApplyObject(ctx context.Context, client k8sclient.Client, obj *uns.Unstructured) error {
	existing, objDesc, err := getExistingObject(ctx, client, obj)

	if err != nil && apierrors.IsNotFound(err) {
		log.Printf("does not exist, creating %s", objDesc)
		err := client.Create(ctx, obj)
		if err != nil {
			return err
		}
		log.Printf("successfully created %s", objDesc)
	}

	if existing == nil {
		return nil
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
