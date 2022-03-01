package tests

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/test/consts"
	testclient "github.com/metallb/metallb-operator/test/e2e/client"
	metallbutils "github.com/metallb/metallb-operator/test/e2e/metallb"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("metallb", func() {
	BeforeEach(createMetalLBResource)
	AfterEach(func() {
		deleteMetalLBResource()
		deleteTestCRs()
	})
	Context("with BFD profile", func() {
		table.DescribeTable("should render the configmap properly", func(objects []client.Object, expectedConfigMap string) {
			By("Creating AddressPool CR")

			for _, obj := range objects {
				By(fmt.Sprintf("Creating the object %s %s %s", obj.GetNamespace(), obj.GetName(), obj.GetObjectKind().GroupVersionKind().Kind))
				Expect(testclient.Client.Create(context.Background(), obj)).Should(Succeed())
			}

			By("Checking ConfigMap is created match the expected configuration")
			Eventually(func() string {
				configmap, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
				if err != nil {
					return ""
				}
				return configmap.Data[consts.MetalLBConfigMapName]
			}, metallbutils.Timeout, metallbutils.Interval).Should(MatchYAML(expectedConfigMap))

			By("Checking that deleting the objects clear the ConfigMap is cleared")
			for _, obj := range objects {
				err := testclient.Client.Delete(context.Background(), obj)
				Expect(err).ToNot(HaveOccurred())
			}

			Eventually(func() string {
				configmap, _ := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})

				return configmap.Data[consts.MetalLBConfigMapName]
			}, metallbutils.Timeout, metallbutils.Interval).Should(MatchYAML("{}"))
		},

			table.Entry("Test two bfd profiles", []client.Object{
				&metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-profile1",
						Namespace: OperatorNameSpace,
					},
					Spec: metallbv1beta1.BFDProfileSpec{
						ReceiveInterval:  uint32Ptr(12),
						TransmitInterval: uint32Ptr(13),
						DetectMultiplier: uint32Ptr(14),
						EchoInterval:     uint32Ptr(15),
						EchoMode:         pointer.BoolPtr(true),
						PassiveMode:      pointer.BoolPtr(true),
						MinimumTTL:       uint32Ptr(16),
					},
				},
				&metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-profile2",
						Namespace: OperatorNameSpace,
					},
					Spec: metallbv1beta1.BFDProfileSpec{},
				},
				&metallbv1beta1.AddressPool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "addresspool1",
						Namespace: OperatorNameSpace,
					},
					Spec: metallbv1beta1.AddressPoolSpec{
						Protocol: "layer2",
						Addresses: []string{
							"1.1.1.1-1.1.1.100",
						},
					},
				},
			},
				`address-pools:
- addresses:
  - 1.1.1.1-1.1.1.100
  name: addresspool1
  protocol: layer2
bfd-profiles:
- detect-multiplier: 14
  echo-mode: true
  echo-interval: 15
  minimum-ttl: 16
  name: test-profile1
  passive-mode: true
  receive-interval: 12
  transmit-interval: 13
- name: test-profile2`))
	})
})

func uint32Ptr(n uint32) *uint32 {
	return &n
}

func createMetalLBResource() {
	metallb := &metallbv1beta1.MetalLB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metallb",
			Namespace: OperatorNameSpace,
		},
	}
	By("Creating a MetalLB resource")
	err := testclient.Client.Create(context.Background(), metallb)
	Expect(err).ToNot(HaveOccurred())
	configmap := &corev1.ConfigMap{}
	Eventually(func() error {
		return testclient.Client.Get(context.Background(), types.NamespacedName{Name: consts.MetalLBConfigMapName, Namespace: OperatorNameSpace}, configmap)
	}, metallbutils.Timeout, metallbutils.Interval).Should(BeNil())
}

func deleteMetalLBResource() {
	By("Deleting MetalLB resource")
	err := testclient.Client.DeleteAllOf(context.Background(), &metallbv1beta1.MetalLB{}, client.InNamespace(OperatorNameSpace))
	Expect(err).ToNot(HaveOccurred())
	Eventually(func() bool {
		_, err = testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
		return errors.IsNotFound(err)
	}, metallbutils.Timeout, metallbutils.Interval).Should(Equal(true))
}
