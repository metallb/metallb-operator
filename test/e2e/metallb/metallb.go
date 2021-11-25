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

// Delete and check the MetalLB custom resource is deleted to avoid status leak in between tests.
func Delete(metallb *metallbv1beta1.MetalLB) {
	err := testclient.Client.Delete(context.Background(), metallb)
	if errors.IsNotFound(err) { // Ignore err, could be already deleted.
		return
	}
	Expect(err).ToNot(HaveOccurred())

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
			LabelSelector: fmt.Sprintf("app.kubernetes.io/component=%s", consts.MetalLBDeploymentName)})
		return len(pods.Items) == 0
	}, DeployTimeout, Interval).Should(BeTrue())

	Eventually(func() bool {
		pods, _ := testclient.Client.Pods(metallb.Namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app.kubernetes.io/component=%s", consts.MetalLBDaemonsetName)})
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
