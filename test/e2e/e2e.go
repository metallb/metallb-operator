package e2e

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	//"k8s.io/apimachinery/pkg/api/errors"
	testclient "github.com/fedepaol/metallboperator/test/e2e/client"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	goclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

const (
	// MetallbNameSpace contains the name of the metal LB operator  namespace
	MetallbNameSpace = "metallb-system"
	// MetallbOperatorDeploymentName contains the name of the metal LB operator deployment
	MetallbOperatorDeploymentName = "metallboperator-controller-manager"
	// MetallbOperatorDeploymentLabel contains the label of the metal LB operator deployment
	MetallbOperatorDeploymentLabel = "controller-manager"
	// MetallbOperatorCRDName contains the name of the metal LB operator CRD
	MetallbOperatorCRDName = "metallbs.metallb.io"
)

func RunE2ETests(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = Describe("validation", func() {
	Context("general", func() {
		It("should have the metal LB operator namespace", func() {
			_, err := testclient.Client.Namespaces().Get(context.Background(), MetallbNameSpace, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should have all the nodes in ready state", func() {
			nodes, err := testclient.Client.Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())

			for _, node := range nodes.Items {
				nodeReady := false
				for _, condition := range node.Status.Conditions {
					if condition.Type == corev1.NodeReady &&
						condition.Status == corev1.ConditionTrue {
						nodeReady = true
					}
				}
				Expect(nodeReady).To(BeTrue(), "Node ", node.Name, " is not ready")
			}
		})
	})

	Context("metallb", func() {
		It("should have the metal LB operator deployment in running state", func() {
			deploy, err := testclient.Client.Deployments(MetallbNameSpace).Get(context.Background(), MetallbOperatorDeploymentName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(deploy.Status.Replicas).To(Equal(deploy.Status.Replicas), "Deployment %s is not ready", MetallbOperatorDeploymentName)

			pods, err := testclient.Client.Pods(MetallbNameSpace).List(context.Background(), metav1.ListOptions{
				LabelSelector: fmt.Sprintf("control-plane=%s", MetallbOperatorDeploymentLabel)})

			Expect(err).ToNot(HaveOccurred())
			Expect(len(pods.Items)).To(Equal(1))
			Expect(pods.Items[0].Status.Phase).To(Equal(corev1.PodRunning))
		})

		It("should have the metal LB operator CRD available in the cluster", func() {
			crd := &apiext.CustomResourceDefinition{}
			err := testclient.Client.Get(context.TODO(), goclient.ObjectKey{Name: MetallbOperatorCRDName}, crd)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
