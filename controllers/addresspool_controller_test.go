package controllers

import (
	"context"
	"time"

	"k8s.io/utils/pointer"

	"github.com/metallb/metallb-operator/api/v1alpha1"
	"github.com/metallb/metallb-operator/pkg/apply"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("AddressPool Controller", func() {
	Context("Creating AddressPool object Layer2 Config", func() {
		autoAssign := false
		configmap := &corev1.ConfigMap{}

		AfterEach(func() {
			err := cleanTestNamespace()
			Expect(err).ToNot(HaveOccurred())
		})
		It("Should create AddressPool Objects", func() {
			addressPool1 := &v1alpha1.AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool1",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: v1alpha1.AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"1.1.1.1-1.1.1.100",
					},
					AutoAssign: &autoAssign,
				},
			}
			addressPool2 := &v1alpha1.AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: v1alpha1.AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"2.2.2.2-2.2.2.100",
					},
					AutoAssign: &autoAssign,
				},
			}
			By("Creating 1st AddressPool resource")
			err := k8sClient.Create(context.Background(), addressPool1)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is created and matches test-addresspool1 configuration")
			Eventually(func() (string, error) {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: apply.MetalLBConfigMap, Namespace: MetalLBTestNameSpace}, configmap)
				if err != nil {
					return "", err
				}
				return configmap.Data[apply.MetalLBConfigMap], err
			}, 2*time.Second, 200*time.Millisecond).Should(MatchYAML(`address-pools:
- name: test-addresspool1
  protocol: layer2
  auto-assign: false
  addresses:
  - 1.1.1.1-1.1.1.100
`))
			By("Creating 2nd AddressPool resource")
			err = k8sClient.Create(context.Background(), addressPool2)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is created and matches test-addresspool1 & 2 configuration")
			Eventually(func() (string, error) {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: apply.MetalLBConfigMap, Namespace: MetalLBTestNameSpace}, configmap)
				if err != nil {
					return "", err
				}
				return configmap.Data[apply.MetalLBConfigMap], err
			}, 2*time.Second, 200*time.Millisecond).Should(MatchYAML(`address-pools:
- name: test-addresspool1
  protocol: layer2
  auto-assign: false
  addresses:
  - 1.1.1.1-1.1.1.100
- name: test-addresspool2
  protocol: layer2
  auto-assign: false
  addresses:
  - 2.2.2.2-2.2.2.100
`))
			By("Deleting 1st AddressPool resource")
			err = k8sClient.Delete(context.Background(), addressPool1)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is created and matches test-addresspool2 configuration")
			Eventually(func() (string, error) {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: apply.MetalLBConfigMap, Namespace: MetalLBTestNameSpace}, configmap)
				if err != nil {
					return "", err
				}
				return configmap.Data[apply.MetalLBConfigMap], err
			}, 2*time.Second, 200*time.Millisecond).Should(MatchYAML(`address-pools:
- name: test-addresspool2
  protocol: layer2
  auto-assign: false
  addresses:
  - 2.2.2.2-2.2.2.100

`))
			By("Deleting 2nd AddressPool resource")
			err = k8sClient.Delete(context.Background(), addressPool2)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is empty")
			Eventually(func() string {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: apply.MetalLBConfigMap, Namespace: MetalLBTestNameSpace}, configmap)
				Expect(err).ToNot(HaveOccurred())
				return configmap.Data["config"]
			}, 2*time.Second, 200*time.Millisecond).Should(MatchYAML("{}"))
		})
	})

	Context("Creating AddressPool object BGP Config", func() {
		autoAssign := false
		configmap := &corev1.ConfigMap{}
		addressPool := &v1alpha1.AddressPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-addresspool-bgp",
				Namespace: MetalLBTestNameSpace,
			},
			Spec: v1alpha1.AddressPoolSpec{
				Protocol: "bgp",
				Addresses: []string{
					"2.2.2.2",
					"2.2.2.100",
				},
				AutoAssign: &autoAssign,
				BGPAdvertisements: []v1alpha1.BgpAdvertisement{
					{
						AggregationLength:   pointer.Int32Ptr(24),
						AggregationLengthV6: pointer.Int32Ptr(124),
						LocalPref:           100,
						Communities: []string{
							"65535:65282",
							"7003:007",
						},
					},
				},
			},
		}

		AfterEach(func() {
			err := cleanTestNamespace()
			Expect(err).ToNot(HaveOccurred())
		})
		It("Should create AddressPool Object", func() {

			By("Creating a AddressPool resource")
			err := k8sClient.Create(context.Background(), addressPool)
			Expect(err).ToNot(HaveOccurred())

			// Checking ConfigMap is created
			By("Checking ConfigMap is created and matches test-addresspool-bgp configuration")
			Eventually(func() (string, error) {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: apply.MetalLBConfigMap, Namespace: MetalLBTestNameSpace}, configmap)
				if err != nil {
					return "", err
				}
				return configmap.Data[apply.MetalLBConfigMap], err
			}, 2*time.Second, 200*time.Millisecond).Should(MatchYAML(`address-pools:
- name: test-addresspool-bgp
  protocol: bgp
  auto-assign: false
  addresses:
  - 2.2.2.2
  - 2.2.2.100
  bgp-advertisements: 
  - communities:
    - 65535:65282
    - 7003:007
    aggregation-length: 24
    aggregation-length-v6: 124
    localpref: 100
`))
		})
	})
})
