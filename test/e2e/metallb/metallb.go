package metallb

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	. "github.com/onsi/gomega"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/pkg/status"
	"github.com/metallb/metallb-operator/test/consts"
	testclient "github.com/metallb/metallb-operator/test/e2e/client"
	v1 "k8s.io/api/core/v1"
	schv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	goclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Timeout and Interval settings
	Timeout       = time.Second * 5
	DeployTimeout = time.Minute * 3
	Interval      = time.Second * 2
)

type CreateOption func(m *metallbv1beta1.MetalLB)

// Delete and check the MetalLB custom resource is deleted to avoid status leak in between tests.
func Delete(metallb *metallbv1beta1.MetalLB) {
	err := testclient.Client.Delete(context.Background(), metallb)
	if errors.IsNotFound(err) { // Ignore err, could be already deleted.
		return
	}
	Expect(err).ToNot(HaveOccurred())
}

func DeleteAndCheck(metallb *metallbv1beta1.MetalLB) {
	Delete(metallb)

	Eventually(func() bool {
		err := testclient.Client.Get(context.Background(), goclient.ObjectKey{Namespace: metallb.Namespace, Name: metallb.Name}, metallb)
		return errors.IsNotFound(err)
	}, 1*time.Minute, 5*time.Second).Should(BeTrue(), "Failed to delete MetalLB custom resource")

	Eventually(func() bool {
		_, err := testclient.Client.Deployments(metallb.Namespace).Get(context.Background(), consts.MetalLBDeploymentName, metav1.GetOptions{})
		return errors.IsNotFound(err)
	}, DeployTimeout, Interval).Should(BeTrue())

	Eventually(func() bool {
		_, err := testclient.Client.DaemonSets(metallb.Namespace).Get(context.Background(), consts.MetalLBDaemonsetName, metav1.GetOptions{})
		return errors.IsNotFound(err)
	}, DeployTimeout, Interval).Should(BeTrue())

	Eventually(func() bool {
		pods, _ := testclient.Client.Pods(metallb.Namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("component=%s", consts.MetalLBDeploymentName)})
		return len(pods.Items) == 0
	}, DeployTimeout, Interval).Should(BeTrue())

	Eventually(func() bool {
		pods, _ := testclient.Client.Pods(metallb.Namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("component=%s", consts.MetalLBDaemonsetName)})
		return len(pods.Items) == 0
	}, DeployTimeout, Interval).Should(BeTrue())
}

func Get(operatorNamespace string, useMetallbResourcesFromFile bool) (*metallbv1beta1.MetalLB, error) {
	metallb := &metallbv1beta1.MetalLB{}
	if useMetallbResourcesFromFile {
		if err := loadFromFile(metallb, consts.MetalLBCRFile); err != nil {
			return nil, err
		}
	} else {
		metallb.SetName("metallb")
	}
	metallb.SetNamespace(operatorNamespace)
	return metallb, nil

}

func New(operatorNamespace string, opts ...CreateOption) *metallbv1beta1.MetalLB {
	metallb := &metallbv1beta1.MetalLB{}
	metallb.SetName("metallb")
	metallb.SetNamespace(operatorNamespace)
	for _, opt := range opts {
		opt(metallb)
	}
	return metallb
}

func CheckConditionStatus(instance *metallbv1beta1.MetalLB) string {
	availableStatus := false
	degradedStatus := false
	for _, condition := range instance.Status.Conditions {
		if condition.Type == status.ConditionDegraded && condition.Status == metav1.ConditionTrue {
			degradedStatus = true
		}
		if condition.Type == status.ConditionAvailable && condition.Status == metav1.ConditionTrue {
			availableStatus = true
		}
	}
	if availableStatus && !degradedStatus {
		return status.ConditionAvailable
	}
	if !availableStatus && degradedStatus {
		return status.ConditionDegraded
	}
	return ""
}

func decodeYAML(r io.Reader, obj interface{}) error {
	decoder := yaml.NewYAMLToJSONDecoder(r)
	return decoder.Decode(obj)
}

func loadFromFile(metallb *metallbv1beta1.MetalLB, fileName string) error {
	f, err := os.Open(fmt.Sprintf("../../../config/samples/%s", fileName))
	if err != nil {
		return err
	}
	defer f.Close()

	return decodeYAML(f, metallb)
}

func NewPriorityClass(name string, priority int32) *schv1.PriorityClass {
	pc := &schv1.PriorityClass{}
	pc.Name = name
	pc.Value = priority
	return pc
}

func DeletePriorityClass(pc *schv1.PriorityClass) {
	err := testclient.Client.Delete(context.Background(), pc)
	if errors.IsNotFound(err) { // Ignore err, could be already deleted.
		return
	}
	Expect(err).ToNot(HaveOccurred())
}

// WaitForControllerDeploymentReady waits for the MetalLB controller deployment to be ready
func WaitForControllerDeploymentReady(namespace string, timeout time.Duration) {
	Eventually(func() error {
		deploy, err := testclient.Client.Deployments(namespace).Get(context.Background(), consts.MetalLBDeploymentName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if deploy.Status.ReadyReplicas != deploy.Status.Replicas {
			return fmt.Errorf("deployment %s is not ready, expected %d ready replicas got %d", consts.MetalLBDeploymentName, deploy.Status.Replicas, deploy.Status.ReadyReplicas)
		}

		return nil
	}, timeout, Interval).ShouldNot(HaveOccurred())
}

// WaitForSpeakerDaemonSetReady waits for the MetalLB speaker daemonset to be ready
func WaitForSpeakerDaemonSetReady(namespace string, timeout time.Duration) {
	Eventually(func() error {
		daemonset, err := testclient.Client.DaemonSets(namespace).Get(context.Background(), consts.MetalLBDaemonsetName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if daemonset.Status.DesiredNumberScheduled == 0 {
			return fmt.Errorf("daemonset %s has no desired pods scheduled", consts.MetalLBDaemonsetName)
		}
		if daemonset.Status.DesiredNumberScheduled != daemonset.Status.NumberReady {
			return fmt.Errorf("daemonset %s is not ready, expected %d ready pods got %d", consts.MetalLBDaemonsetName, daemonset.Status.DesiredNumberScheduled, daemonset.Status.NumberReady)
		}

		return nil
	}, timeout, Interval).ShouldNot(HaveOccurred())
}

// WaitForProgressingConditionTrue waits for the MetalLB Progressing condition to be True
func WaitForProgressingConditionTrue(metallb *metallbv1beta1.MetalLB, timeout time.Duration) {
	Eventually(func() bool {
		config := &metallbv1beta1.MetalLB{}
		err := testclient.Client.Get(context.Background(), goclient.ObjectKey{Namespace: metallb.Namespace, Name: metallb.Name}, config)
		if err != nil {
			return false
		}
		if config.Status.Conditions == nil {
			return false
		}
		for _, condition := range config.Status.Conditions {
			if condition.Type == status.ConditionProgressing && condition.Status == metav1.ConditionTrue {
				return true
			}
		}
		return false
	}, timeout, Interval).Should(BeTrue(), "Progressing condition should be True after update")
}

// WaitForProgressingFalseAndAvailableTrue waits for the MetalLB Progressing condition to be False and Available condition to be True
func WaitForProgressingFalseAndAvailableTrue(metallb *metallbv1beta1.MetalLB, timeout time.Duration) {
	Eventually(func() bool {
		config := &metallbv1beta1.MetalLB{}
		err := testclient.Client.Get(context.Background(), goclient.ObjectKey{Namespace: metallb.Namespace, Name: metallb.Name}, config)
		if err != nil {
			return false
		}
		if config.Status.Conditions == nil {
			return false
		}
		progressingFalse := false
		availableTrue := false
		for _, condition := range config.Status.Conditions {
			if condition.Type == status.ConditionProgressing && condition.Status == metav1.ConditionFalse {
				progressingFalse = true
			}
			if condition.Type == status.ConditionAvailable && condition.Status == metav1.ConditionTrue {
				availableTrue = true
			}
		}
		return progressingFalse && availableTrue
	}, timeout, Interval).Should(BeTrue(), "Progressing should be False and Available should be True after update completes")
}

func DeleteDefaultMetalLB(namespace string, useMetallbResourcesFromFile bool) {
	metallb, err := Get(namespace, useMetallbResourcesFromFile)
	Expect(err).ToNot(HaveOccurred(), "Failed to get MetalLB resource")
	DeleteAndCheck(metallb)
}

// getMetalLBPods fetches pods for a given MetalLB component (controller or speaker)
// and validates that the list is not empty
func getMetalLBPods(namespace string, component string) *v1.PodList {
	pods, err := testclient.Client.Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("component=%s", component),
	})
	Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Failed to list %s pods", component))
	Expect(len(pods.Items)).Should(BeNumerically(">", 0), fmt.Sprintf("%s Pods List should not be empty", component))
	return pods
}

// GetMetalLBControllerPods fetches the controller pods for a given namespace
func GetMetalLBControllerPods(namespace string) *v1.PodList {
	return getMetalLBPods(namespace, "controller")
}

// GetMetalLBSpeakerPods fetches the speaker pods for a given namespace
func GetMetalLBSpeakerPods(namespace string) *v1.PodList {
	return getMetalLBPods(namespace, "speaker")
}
