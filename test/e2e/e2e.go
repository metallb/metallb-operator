package e2e

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metallbv1alpha "github.com/metallb/metallb-operator/api/v1alpha1"
	testclient "github.com/metallb/metallb-operator/test/e2e/client"
	corev1 "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	goclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

const (
	// MetallbNameSpace contains the name of the MetalLB operator  namespace
	MetallbNameSpace = "metallb-system"
	// MetallbOperatorDeploymentName contains the name of the MetalLB operator deployment
	MetallbOperatorDeploymentName = "metallboperator-controller-manager"
	// MetallbOperatorDeploymentLabel contains the label of the MetalLB operator deployment
	MetallbOperatorDeploymentLabel = "controller-manager"
	// MetallbOperatorCRDName contains the name of the MetalLB operator CRD
	MetallbOperatorCRDName = "metallbs.metallb.io"
	// MetallbCRFile contains the Metallb custom resource deployment
	MetallbCRFile = "metallb.yaml"
	// MetallbDeploymentName contains the name of the MetalLB deployment
	MetallbDeploymentName = "controller"
	// MetallbDaemonsetName contains the name of the MetalLB daemonset
	MetallbDaemonsetName = "speaker"
)

func RunE2ETests(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = Describe("validation", func() {
	Context("MetalLB", func() {
		It("should have the MetalLB operator deployment in running state", func() {
			Eventually(func() bool {
				deploy, err := testclient.Client.Deployments(MetallbNameSpace).Get(context.Background(), MetallbOperatorDeploymentName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return deploy.Status.ReadyReplicas == deploy.Status.Replicas
			}, 5*time.Minute, 5*time.Second).Should(BeTrue())

			pods, err := testclient.Client.Pods(MetallbNameSpace).List(context.Background(), metav1.ListOptions{
				LabelSelector: fmt.Sprintf("control-plane=%s", MetallbOperatorDeploymentLabel)})
			Expect(err).ToNot(HaveOccurred())

			deploy, err := testclient.Client.Deployments(MetallbNameSpace).Get(context.Background(), MetallbOperatorDeploymentName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(pods.Items)).To(Equal(int(deploy.Status.Replicas)))

			for _, pod := range pods.Items {
				Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
			}
		})

		It("should have the MetalLB CRD available in the cluster", func() {
			crd := &apiext.CustomResourceDefinition{}
			err := testclient.Client.Get(context.Background(), goclient.ObjectKey{Name: MetallbOperatorCRDName}, crd)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("MetalLB deploy", func() {
		var metallb *metallbv1alpha.Metallb
		var metallbCRExisted bool

		BeforeEach(func() {
			metallb = &metallbv1alpha.Metallb{}
			err := loadMetallbFromFile(metallb, MetallbCRFile)
			Expect(err).ToNot(HaveOccurred())

			metallbCRExisted = true
			err = testclient.Client.Get(context.Background(), goclient.ObjectKey{Namespace: metallb.Namespace, Name: metallb.Name}, metallb)
			if errors.IsNotFound(err) {
				metallbCRExisted = false
				Expect(testclient.Client.Create(context.Background(), metallb)).Should(Succeed())
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
		})

		AfterEach(func() {
			if !metallbCRExisted {
				err := testclient.Client.Delete(context.Background(), metallb)
				Expect(err).ToNot(HaveOccurred())
				// Check the MetalLB custom resource is deleted to avoid status leak in between tests.
				Eventually(func() bool {
					err = testclient.Client.Get(context.Background(), goclient.ObjectKey{Namespace: metallb.Namespace, Name: metallb.Name}, metallb)
					if errors.IsNotFound(err) {
						return true
					}
					return false
				}, 5*time.Minute, 5*time.Second).Should(BeTrue(), "Failed to delete MetalLB custom resource")
			}
		})

		It("should have MetalLB pods in running state", func() {
			By("checking MetalLB controller deployment is in running state", func() {
				Eventually(func() bool {
					deploy, err := testclient.Client.Deployments(metallb.Namespace).Get(context.Background(), MetallbDeploymentName, metav1.GetOptions{})
					if err != nil {
						return false
					}
					return deploy.Status.ReadyReplicas == deploy.Status.Replicas
				}, 5*time.Minute, 5*time.Second).Should(BeTrue())

				pods, err := testclient.Client.Pods(MetallbNameSpace).List(context.Background(), metav1.ListOptions{
					LabelSelector: "component=controller"})
				Expect(err).ToNot(HaveOccurred())

				deploy, err := testclient.Client.Deployments(metallb.Namespace).Get(context.Background(), MetallbDeploymentName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(len(pods.Items)).To(Equal(int(deploy.Status.Replicas)))

				for _, pod := range pods.Items {
					Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
				}
			})

			By("checking MetalLB daemonset is in running state", func() {
				Eventually(func() bool {
					daemonset, err := testclient.Client.DaemonSets(metallb.Namespace).Get(context.Background(), MetallbDaemonsetName, metav1.GetOptions{})
					if err != nil {
						return false
					}
					return daemonset.Status.DesiredNumberScheduled == daemonset.Status.NumberReady
				}, 5*time.Minute, 5*time.Second).Should(BeTrue())

				pods, err := testclient.Client.Pods(MetallbNameSpace).List(context.Background(), metav1.ListOptions{
					LabelSelector: "component=speaker"})
				Expect(err).ToNot(HaveOccurred())

				daemonset, err := testclient.Client.DaemonSets(metallb.Namespace).Get(context.Background(), MetallbDaemonsetName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(len(pods.Items)).To(Equal(int(daemonset.Status.DesiredNumberScheduled)))

				for _, pod := range pods.Items {
					Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
				}
			})
		})
	})
})

func decodeYAML(r io.Reader, obj interface{}) error {
	decoder := yaml.NewYAMLToJSONDecoder(r)
	return decoder.Decode(obj)
}

func loadMetallbFromFile(metallb *metallbv1alpha.Metallb, fileName string) error {
	f, err := os.Open(fmt.Sprintf("../../config/samples/%s", fileName))
	if err != nil {
		return err
	}
	defer f.Close()

	return decodeYAML(f, metallb)
}

var _ = BeforeSuite(func() {
	_, err := testclient.Client.Namespaces().Get(context.Background(), MetallbNameSpace, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred(), "Should have the MetalLB operator namespace")
})
