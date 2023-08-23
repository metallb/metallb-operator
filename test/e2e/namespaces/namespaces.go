package namespaces

import (
	"context"
	"fmt"
	"time"

	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"

	testclient "github.com/metallb/metallb-operator/test/e2e/client"
)

// Test is the namespace to be use for testing
//const Test = "sriov-conformance-testing"

// WaitForDeletion waits until the namespace will be removed from the cluster
func WaitForDeletion(cs *testclient.ClientSet, nsName string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(context.Background(), time.Second, timeout, true, func(c context.Context) (bool, error) {
		_, err := cs.Namespaces().Get(context.Background(), nsName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
}

// Create creates a new namespace with the given name.
// If the namespace exists, it returns.
func Create(namespace string, cs *testclient.ClientSet) error {
	_, err := cs.Namespaces().Create(context.Background(), &k8sv1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		}}, metav1.CreateOptions{})

	if k8serrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

// DeleteAndWait deletes a namespace and waits until delete
func DeleteAndWait(cs *testclient.ClientSet, namespace string, timeout time.Duration) error {
	err := cs.Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return WaitForDeletion(cs, namespace, timeout)
}

// Exists tells whether the given namespace exists
func Exists(namespace string, cs *testclient.ClientSet) bool {
	_, err := cs.Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	return err == nil || !k8serrors.IsNotFound(err)
}

// CleanPods deletes all pods in namespace
func CleanPods(namespace string, cs *testclient.ClientSet) error {
	if !Exists(namespace, cs) {
		return nil
	}
	err := cs.Pods(namespace).DeleteCollection(context.Background(), metav1.DeleteOptions{
		GracePeriodSeconds: pointer.Int64(0),
	}, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("Failed to delete pods %v", err)
	}
	return err
}

// Clean cleans all dangling objects from the given namespace.
func Clean(operatorNamespace, namespace string, cs *testclient.ClientSet, discoveryEnabled bool) error {
	err := CleanPods(namespace, cs)
	if err != nil {
		return err
	}
	if discoveryEnabled {
		return nil
	}
	return nil
}
