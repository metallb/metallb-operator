// SPDX-License-Identifier:Apache-2.0

package k8sreporter

import (
	"log"

	"github.com/openshift-kni/k8sreporter"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/metallb/metallb-operator/api/v1beta1"
)

const MetalLBTestNameSpace = "metallb-test-namespace"

func New(kubeconfig, path, namespace string) *k8sreporter.KubernetesReporter {
	// When using custom crds, we need to add them to the scheme
	addToScheme := func(s *runtime.Scheme) error {
		err := v1beta1.AddToScheme(s)
		if err != nil {
			return err
		}
		return nil
	}

	// The namespaces we want to dump resources for (including pods and pod logs)
	dumpNamespace := func(ns string) bool {
		switch {
		case ns == namespace:
			return true
		case ns == MetalLBTestNameSpace:
			return true
		}
		return false
	}

	// The list of CRDs we want to dump
	crds := []k8sreporter.CRData{
		{Cr: &v1beta1.MetalLBList{}},
		{Cr: &corev1.ServiceList{}},
	}

	reporter, err := k8sreporter.New(kubeconfig, addToScheme, dumpNamespace, path, crds...)
	if err != nil {
		log.Fatalf("Failed to initialize the reporter %s", err)
	}
	return reporter
}
