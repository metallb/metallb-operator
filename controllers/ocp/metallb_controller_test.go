package ocp

import (
	"context"
	"slices"
	"time"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/test/consts"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	openshiftapiv1 "github.com/openshift/api/operator/v1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const cnoVersionSupportingFRRK8s = "4.19.0"

var _ = Describe("MetalLB Controller in openshift mode", func() {
	var (
		networkCR *openshiftapiv1.Network
		cnoCR     *openshiftconfigv1.ClusterOperator
	)

	BeforeEach(func() {
		cnoCR = &openshiftconfigv1.ClusterOperator{
			ObjectMeta: metav1.ObjectMeta{Name: "network"},
		}
		err := k8sClient.Create(context.Background(), cnoCR)
		Expect(err).NotTo(HaveOccurred())

		cnoCR.Status = openshiftconfigv1.ClusterOperatorStatus{
			Versions: []openshiftconfigv1.OperandVersion{
				{Name: "operator", Version: cnoVersionSupportingFRRK8s},
			},
		}
		err = k8sClient.Status().Update(context.Background(), cnoCR)
		Expect(err).NotTo(HaveOccurred())

		networkCR = &openshiftapiv1.Network{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			Spec: openshiftapiv1.NetworkSpec{
				OperatorSpec: openshiftapiv1.OperatorSpec{
					ManagementState: openshiftapiv1.Managed,
				},
			},
		}
		err = k8sClient.Create(context.Background(), networkCR)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_ = k8sClient.Delete(context.Background(), networkCR)
		_ = k8sClient.Delete(context.Background(), cnoCR)

		err := cleanTestNamespace()
		Expect(err).NotTo(HaveOccurred())
	})

	It("should set FRR provider on Network resource and not deploy frr-k8s directly", func() {
		By("Creating MetalLB CR in external mode")
		createMetalLBCR()

		By("Verifying Network CR has FRR provider set")
		Eventually(frrProviderIsSet, 10*time.Second, 200*time.Millisecond).Should(BeTrue())

		By("Verifying frr-k8s DaemonSet is NOT deployed in MetalLB namespace")
		Consistently(frrk8sDaemonSetIsGone, 3*time.Second, 500*time.Millisecond).Should(BeTrue())
	})
})

func createMetalLBCR() {
	metallbCR := &metallbv1beta1.MetalLB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metallb",
			Namespace: MetalLBTestNameSpace,
		},
	}
	Expect(k8sClient.Create(context.Background(), metallbCR)).To(Succeed())
}

func frrProviderIsSet() bool {
	network := &openshiftapiv1.Network{}
	err := k8sClient.Get(context.Background(), types.NamespacedName{Name: "cluster"}, network)
	if err != nil || network.Spec.AdditionalRoutingCapabilities == nil {
		return false
	}
	return slices.Contains(network.Spec.AdditionalRoutingCapabilities.Providers, openshiftapiv1.RoutingCapabilitiesProviderFRR)
}

func frrk8sDaemonSetIsGone() bool {
	ds := &appsv1.DaemonSet{}
	err := k8sClient.Get(context.Background(),
		types.NamespacedName{Name: consts.FRRK8SDaemonsetName, Namespace: MetalLBTestNameSpace}, ds)
	return apierrors.IsNotFound(err)
}

func cleanTestNamespace() error {
	err := k8sClient.DeleteAllOf(context.Background(), &metallbv1beta1.MetalLB{}, client.InNamespace(MetalLBTestNameSpace))
	if err != nil {
		return err
	}
	err = k8sClient.DeleteAllOf(context.Background(), &appsv1.Deployment{}, client.InNamespace(MetalLBTestNameSpace))
	if err != nil {
		return err
	}
	return k8sClient.DeleteAllOf(context.Background(), &appsv1.DaemonSet{}, client.InNamespace(MetalLBTestNameSpace))
}
