package override

import (
	metallbv1alpha1 "github.com/metallb/metallb-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const defaultMetallbNamespace = "metallb-system"

func ResourceOverrides(objs []*unstructured.Unstructured, config *metallbv1alpha1.Metallb) {
	for _, obj := range objs {
		updateNamespace(obj, config)
	}
	return
}

func updateNamespace(obj *unstructured.Unstructured, config *metallbv1alpha1.Metallb) {
	if config.Spec.MetallbNamespace == "" || config.Spec.MetallbNamespace == defaultMetallbNamespace {
		return
	}
	if obj.GetKind() == "Namespace" {
		obj.SetName(config.Spec.MetallbNamespace)
	} else {
		obj.SetNamespace(config.Spec.MetallbNamespace)
	}
	return
}
