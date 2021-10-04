package tests

import (
	"context"
	"fmt"
	"os"

	ctrl "sigs.k8s.io/controller-runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/metallb/metallb-operator/pkg/platform"
	"github.com/metallb/metallb-operator/test/consts"
	testclient "github.com/metallb/metallb-operator/test/e2e/client"
	"github.com/metallb/metallb-operator/test/e2e/metallb"
	corev1 "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	goclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var TestIsOpenShift = false

var OperatorNameSpace = consts.DefaultOperatorNameSpace

func init() {
	if len(os.Getenv("IS_OPENSHIFT")) != 0 {
		TestIsOpenShift = true
	}

	if ns := os.Getenv("OO_INSTALL_NAMESPACE"); len(ns) != 0 {
		OperatorNameSpace = ns
	}
}

var _ = Describe("metallb", func() {
	Context("Platform Check", func() {
		It("Should have the MetalLB Operator namespace", func() {
			_, err := testclient.Client.Namespaces().Get(context.Background(), OperatorNameSpace, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred(), "Should have the MetalLB Operator namespace")
		})
		It("should be either Kubernetes or OpenShift platform", func() {
			cfg := ctrl.GetConfigOrDie()
			platforminfo, err := platform.GetPlatformInfo(cfg)
			Expect(err).ToNot(HaveOccurred())
			Expect(platforminfo.IsOpenShift()).Should(Equal(TestIsOpenShift))
		})
	})

	Context("MetalLB", func() {
		It("should have the MetalLB Operator deployment in running state", func() {
			Eventually(func() bool {
				deploy, err := testclient.Client.Deployments(OperatorNameSpace).Get(context.Background(), consts.MetalLBOperatorDeploymentName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return deploy.Status.ReadyReplicas == deploy.Status.Replicas
			}, metallb.Timeout, metallb.Interval).Should(BeTrue())

			pods, err := testclient.Client.Pods(OperatorNameSpace).List(context.Background(), metav1.ListOptions{
				LabelSelector: fmt.Sprintf("control-plane=%s", consts.MetalLBOperatorDeploymentLabel)})
			Expect(err).ToNot(HaveOccurred())

			deploy, err := testclient.Client.Deployments(OperatorNameSpace).Get(context.Background(), consts.MetalLBOperatorDeploymentName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(pods.Items)).To(Equal(int(deploy.Status.Replicas)))

			for _, pod := range pods.Items {
				Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
			}
		})

		It("should have the MetalLB CRD available in the cluster", func() {
			crd := &apiext.CustomResourceDefinition{}
			err := testclient.Client.Get(context.Background(), goclient.ObjectKey{Name: consts.MetalLBOperatorCRDName}, crd)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should have the MetalLB AddressPool CRD available in the cluster", func() {
			crd := &apiext.CustomResourceDefinition{}
			err := testclient.Client.Get(context.Background(), goclient.ObjectKey{Name: consts.MetalLBAddressPoolCRDName}, crd)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should have the MetalLB BGPPeer CRD available in the cluster", func() {
			crd := &apiext.CustomResourceDefinition{}
			err := testclient.Client.Get(context.Background(), goclient.ObjectKey{Name: consts.MetalLBPeerCRDName}, crd)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
