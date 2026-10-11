package ocp

import (
	"context"
	"slices"
	"strings"
	"time"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/test/consts"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	openshiftapiv1 "github.com/openshift/api/operator/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	networkCRDName             = "networks.operator.openshift.io"
	additionalRoutingField     = "additionalRoutingCapabilities"
	cnoVersionSupportingFRRK8s = "4.19.0"
)

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

	It("should retry in an upgrade scenario until CRD is updated, operator before CNO", func() {
		By("Getting the Network CRD")
		crd := &apiextensionsv1.CustomResourceDefinition{}
		err := k8sClient.Get(context.Background(), types.NamespacedName{Name: networkCRDName}, crd)
		Expect(err).NotTo(HaveOccurred())

		originalSpec := *crd.Spec.DeepCopy()
		DeferCleanup(func() {
			By("Restoring original CRD at test end")
			_ = setNetworkCRDSpec(originalSpec)
		})

		By("Removing additionalRoutingCapabilities from CRD schema (simulates 4.18 CRD)")
		err = setNetworkCRDSpec(specWithoutAdditionalRouting(originalSpec))
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for the API server to reject additionalRoutingCapabilities")
		Eventually(func(g Gomega) {
			network := &openshiftapiv1.Network{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(networkCR), network)).To(Succeed())
			network.Spec.AdditionalRoutingCapabilities = &openshiftapiv1.AdditionalRoutingCapabilities{
				Providers: []openshiftapiv1.RoutingCapabilitiesProvider{openshiftapiv1.RoutingCapabilitiesProviderFRR},
			}
			err := k8sClient.Update(ctx, network, client.DryRunAll, client.FieldValidation(metav1.FieldValidationStrict))
			g.Expect(err).To(MatchError(ContainSubstring(`unknown field "spec.` + additionalRoutingField + `"`)))
			g.Expect(apierrors.IsBadRequest(err)).To(BeTrue())
		}, 10*time.Second, 200*time.Millisecond).Should(Succeed())

		By("Creating old frr-k8s DaemonSet (simulates pre-upgrade state)")
		createFRRK8sDaemonSet()

		By("Creating MetalLB CR")
		createMetalLBCR()

		By("Verifying old frr-k8s DaemonSet is NOT deleted")
		Consistently(func() bool {
			return !frrk8sDaemonSetIsGone()
		}, 3*time.Second, 500*time.Millisecond).Should(BeTrue())

		By("Verifying Network CR does NOT have FRR provider set")
		Expect(frrProviderIsSet()).To(BeFalse())

		By("Updating CRD to schema containing the additionalRouting field")
		err = setNetworkCRDSpec(originalSpec)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying Network CR has FRR provider set")
		Eventually(frrProviderIsSet, 10*time.Second, 200*time.Millisecond).Should(BeTrue())

		By("Verifying old frr-k8s DaemonSet is deleted")
		Eventually(frrk8sDaemonSetIsGone, 10*time.Second, 200*time.Millisecond).Should(BeTrue())
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

func createFRRK8sDaemonSet() {
	labels := map[string]string{"app": consts.FRRK8SDaemonsetName}
	frrk8sDS := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      consts.FRRK8SDaemonsetName,
			Namespace: MetalLBTestNameSpace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Name: consts.FRRK8SDaemonsetName, Image: "test"},
					},
				},
			},
		},
	}
	Expect(k8sClient.Create(context.Background(), frrk8sDS)).To(Succeed())
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

// specWithoutAdditionalRouting returns a copy of the given CRD spec with the
// additionalRoutingCapabilities field (and the validations referring to it) stripped off,
// the way the CRD looks like before the cluster is upgraded.
func specWithoutAdditionalRouting(spec apiextensionsv1.CustomResourceDefinitionSpec) apiextensionsv1.CustomResourceDefinitionSpec {
	res := *spec.DeepCopy()
	for i, v := range res.Versions {
		if v.Schema == nil || v.Schema.OpenAPIV3Schema == nil {
			continue
		}
		specProps, ok := v.Schema.OpenAPIV3Schema.Properties["spec"]
		if !ok {
			continue
		}

		delete(specProps.Properties, additionalRoutingField)
		specProps.XValidations = slices.DeleteFunc(specProps.XValidations, func(rule apiextensionsv1.ValidationRule) bool {
			return strings.Contains(rule.Rule, additionalRoutingField)
		})

		res.Versions[i].Schema.OpenAPIV3Schema.Properties["spec"] = specProps
	}
	return res
}

// setNetworkCRDSpec overwrites the spec of the openshift Network CRD, allowing the tests to
// simulate a cluster whose CRDs were not upgraded yet.
func setNetworkCRDSpec(spec apiextensionsv1.CustomResourceDefinitionSpec) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		err := k8sClient.Get(context.Background(), types.NamespacedName{Name: networkCRDName}, crd)
		if err != nil {
			return err
		}
		crd.Spec = *spec.DeepCopy()
		return k8sClient.Update(context.Background(), crd)
	})
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
