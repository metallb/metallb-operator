package controllers

import (
	"context"
	"time"

	"github.com/metallb/metallb-operator/api/v1alpha1"
	"github.com/metallb/metallb-operator/pkg/apply"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Peer Controller", func() {
	Context("Creating Peer object", func() {
		configmap := &corev1.ConfigMap{}

		AfterEach(func() {
			err := cleanTestNamespace()
			Expect(err).ToNot(HaveOccurred())
		})
		It("Should create Peer Objects", func() {
			Peer1 := &v1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bgp-peer1",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: v1alpha1.BGPPeerSpec{
					Address:  "10.0.0.1",
					ASN:      64501,
					MyASN:    64500,
					RouterID: "10.10.10.10",
					NodeSelectors: []v1alpha1.NodeSelector{
						{
							MatchExpressions: []v1alpha1.MatchExpression{
								{
									Key:      "kubernetes.io/hostname",
									Operator: "In",
									Values: []string{
										"hostA",
										"hostB",
									},
								},
							},
						},
					},
				},
			}

			Peer2 := &v1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bgp-peer2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: v1alpha1.BGPPeerSpec{
					Address:  "11.0.0.1",
					ASN:      64001,
					MyASN:    64000,
					RouterID: "11.11.11.11",
				},
			}
			By("Creating 1st BGPPeer resource")
			err := k8sClient.Create(context.Background(), Peer1)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is created and matches bgp-peer1 configuration")
			Eventually(func() (string, error) {
				err := k8sClient.Get(context.Background(),
					types.NamespacedName{Name: apply.MetalLBConfigMap, Namespace: MetalLBTestNameSpace}, configmap)
				if err != nil {
					return "", err
				}
				return configmap.Data[apply.MetalLBConfigMap], err
			}, 2*time.Second, 200*time.Millisecond).Should(MatchYAML(`peers:
- my-asn: 64500
  node-selectors:
  - match-expressions:
    - key: kubernetes.io/hostname
      operator: In
      values:
      - hostA hostB
  peer-address: 10.0.0.1
  peer-asn: 64501
  router-id: 10.10.10.10
`))
			By("Creating 2nd BGPPeer resource")
			err = k8sClient.Create(context.Background(), Peer2)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is created and matches bgp-peer1 & 2 configuration")
			Eventually(func() (string, error) {
				err := k8sClient.Get(context.Background(),
					types.NamespacedName{Name: apply.MetalLBConfigMap, Namespace: MetalLBTestNameSpace}, configmap)
				if err != nil {
					return "", err
				}
				return configmap.Data[apply.MetalLBConfigMap], err
			}, 2*time.Second, 200*time.Millisecond).Should(MatchYAML(`peers:
- my-asn: 64500
  node-selectors:
  - match-expressions:
    - key: kubernetes.io/hostname
      operator: In
      values:
      - hostA hostB
  peer-address: 10.0.0.1
  peer-asn: 64501
  router-id: 10.10.10.10
- my-asn: 64000
  peer-address: 11.0.0.1
  peer-asn: 64001
  router-id: 11.11.11.11
`))

			By("Deleting 1st BGPPeer resource")
			err = k8sClient.Delete(context.Background(), Peer1)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap matches bgp-peer2 configuration")
			Eventually(func() (string, error) {
				err := k8sClient.Get(context.Background(),
					types.NamespacedName{Name: apply.MetalLBConfigMap, Namespace: MetalLBTestNameSpace}, configmap)
				if err != nil {
					return "", err
				}
				return configmap.Data[apply.MetalLBConfigMap], err
			}, 2*time.Second, 200*time.Millisecond).Should(MatchYAML(`peers:
- my-asn: 64000
  peer-address: 11.0.0.1
  peer-asn: 64001
  router-id: 11.11.11.11
`))
			By("Deleting 2nd BGPPeer resource")
			err = k8sClient.Delete(context.Background(), Peer2)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap deleted")
			Eventually(func() (string, error) {
				err := k8sClient.Get(context.Background(),
					types.NamespacedName{Name: apply.MetalLBConfigMap, Namespace: MetalLBTestNameSpace}, configmap)
				if err != nil {
					// if its notfound means that was the last object and configmap is deleted
					if errors.IsNotFound(err) {
						return "", nil
					}
					return "", err
				}
				return configmap.Data[apply.MetalLBConfigMap], err
			}, 2*time.Second, 200*time.Millisecond).Should(MatchYAML(``))
		})
	})

	Context("Creating Full BGP configuration", func() {
		configmap := &corev1.ConfigMap{}

		AfterEach(func() {
			err := cleanTestNamespace()
			Expect(err).ToNot(HaveOccurred())
		})
		It("Should create BGP Configuration Objects", func() {
			autoAssign := false
			addressPool1 := &v1alpha1.AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool1",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: v1alpha1.AddressPoolSpec{
					Protocol: "bgp",
					Addresses: []string{
						"1.1.1.1-1.1.1.100",
					},
					AutoAssign: &autoAssign,
					BGPAdvertisements: []v1alpha1.BgpAdvertisement{
						{
							AggregationLength: 24,
							LocalPref:         100,
							Communities: []string{
								"65535:65282",
								"7003:007",
							},
						},
					},
				},
			}
			addressPool2 := &v1alpha1.AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: v1alpha1.AddressPoolSpec{
					Protocol: "bgp",
					Addresses: []string{
						"2.2.2.2-2.2.2.100",
					},
					AutoAssign: &autoAssign,
				},
			}

			Peer1 := &v1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bgp-peer1",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: v1alpha1.BGPPeerSpec{
					Address:  "10.0.0.1",
					ASN:      64501,
					MyASN:    64500,
					RouterID: "10.10.10.10",
				},
			}
			Peer2 := &v1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bgp-peer2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: v1alpha1.BGPPeerSpec{
					Address:  "11.0.0.1",
					ASN:      64001,
					MyASN:    64000,
					RouterID: "11.11.11.11",
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
  protocol: bgp
  addresses:
  - 1.1.1.1-1.1.1.100
  auto-assign: false
  bgp-advertisements: 
  - communities: 
    - 65535:65282
    - 7003:007
    aggregation-length: 24
    localpref: 100
`))
			By("Creating 1st BGPPeer resource")
			err = k8sClient.Create(context.Background(), Peer1)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is created and matches test-addresspool1 and bgp-peer1 configuration")
			Eventually(func() (string, error) {
				err := k8sClient.Get(context.Background(),
					types.NamespacedName{Name: apply.MetalLBConfigMap, Namespace: MetalLBTestNameSpace}, configmap)
				if err != nil {
					return "", err
				}
				return configmap.Data[apply.MetalLBConfigMap], err
			}, 2*time.Second, 200*time.Millisecond).Should(MatchYAML(`address-pools:
- name: test-addresspool1
  protocol: bgp
  addresses:
  - 1.1.1.1-1.1.1.100
  auto-assign: false
  bgp-advertisements: 
  - communities: 
    - 65535:65282
    - 7003:007
    aggregation-length: 24
    localpref: 100
peers:
- my-asn: 64500
  peer-address: 10.0.0.1
  peer-asn: 64501
  router-id: 10.10.10.10
`))
			By("Creating 2nd AddressPool resource")
			err = k8sClient.Create(context.Background(), addressPool2)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is created and matches test-addresspool1,2 and bgp-peer1 configuration")
			Eventually(func() (string, error) {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: apply.MetalLBConfigMap, Namespace: MetalLBTestNameSpace}, configmap)
				if err != nil {
					return "", err
				}
				return configmap.Data[apply.MetalLBConfigMap], err
			}, 2*time.Second, 200*time.Millisecond).Should(MatchYAML(`address-pools:
- name: test-addresspool1
  protocol: bgp
  addresses:
  - 1.1.1.1-1.1.1.100
  auto-assign: false
  bgp-advertisements: 
  - communities: 
    - 65535:65282
    - 7003:007
    localpref: 100
    aggregation-length: 24
- name: test-addresspool2
  protocol: bgp
  addresses:
  - 2.2.2.2-2.2.2.100
  auto-assign: false
peers:
- my-asn: 64500
  peer-address: 10.0.0.1
  peer-asn: 64501
  router-id: 10.10.10.10
`))
			By("Creating 2nd BGPPeer resource")
			err = k8sClient.Create(context.Background(), Peer2)
			Expect(err).ToNot(HaveOccurred())
			By("Checking ConfigMap is created and matches test-addresspool1,2 and bgp-peer1,2 configuration")
			Eventually(func() (string, error) {
				err := k8sClient.Get(context.Background(),
					types.NamespacedName{Name: apply.MetalLBConfigMap, Namespace: MetalLBTestNameSpace}, configmap)
				if err != nil {
					return "", err
				}
				return configmap.Data[apply.MetalLBConfigMap], err
			}, 2*time.Second, 200*time.Millisecond).Should(MatchYAML(`address-pools:
- name: test-addresspool1
  protocol: bgp
  addresses:
  - 1.1.1.1-1.1.1.100
  auto-assign: false
  bgp-advertisements: 
  - communities: 
    - 65535:65282
    - 7003:007
    localpref: 100
    aggregation-length: 24
- name: test-addresspool2
  protocol: bgp
  addresses:
  - 2.2.2.2-2.2.2.100
  auto-assign: false
peers:
- my-asn: 64500
  peer-address: 10.0.0.1
  peer-asn: 64501
  router-id: 10.10.10.10
- my-asn: 64000
  peer-address: 11.0.0.1
  peer-asn: 64001
  router-id: 11.11.11.11
`))
			By("Deleting 1st BGPPeer resource")
			err = k8sClient.Delete(context.Background(), Peer1)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap matches configuration")
			Eventually(func() (string, error) {
				err := k8sClient.Get(context.Background(),
					types.NamespacedName{Name: apply.MetalLBConfigMap, Namespace: MetalLBTestNameSpace}, configmap)
				if err != nil {
					return "", err
				}
				return configmap.Data[apply.MetalLBConfigMap], err
			}, 2*time.Second, 200*time.Millisecond).Should(MatchYAML(`address-pools:
- name: test-addresspool1
  protocol: bgp
  addresses:
  - 1.1.1.1-1.1.1.100
  auto-assign: false
  bgp-advertisements: 
  - communities: 
    - 65535:65282
    - 7003:007
    localpref: 100
    aggregation-length: 24
- name: test-addresspool2
  protocol: bgp
  addresses:
  - 2.2.2.2-2.2.2.100
  auto-assign: false
peers:
- my-asn: 64000
  peer-address: 11.0.0.1
  peer-asn: 64001
  router-id: 11.11.11.11
`))
			By("Deleting 1st AddressPool resource")
			err = k8sClient.Delete(context.Background(), addressPool1)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is created and matches test-addresspool2 and bgp-peer2 configuration")
			Eventually(func() (string, error) {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: apply.MetalLBConfigMap, Namespace: MetalLBTestNameSpace}, configmap)
				if err != nil {
					return "", err
				}
				return configmap.Data[apply.MetalLBConfigMap], err
			}, 2*time.Second, 200*time.Millisecond).Should(MatchYAML(`address-pools:
- name: test-addresspool2
  protocol: bgp
  addresses:
  - 2.2.2.2-2.2.2.100
  auto-assign: false
peers:
- my-asn: 64000
  peer-address: 11.0.0.1
  peer-asn: 64001
  router-id: 11.11.11.11
`))
			By("Deleting 2nd BGPPeer resource")
			err = k8sClient.Delete(context.Background(), Peer2)
			Expect(err).ToNot(HaveOccurred())

			By("Checking all peers configuration is deleted and test-addresspool2 is still there")
			Eventually(func() (string, error) {
				err := k8sClient.Get(context.Background(),
					types.NamespacedName{Name: apply.MetalLBConfigMap, Namespace: MetalLBTestNameSpace}, configmap)
				if err != nil {
					if errors.IsNotFound(err) {
						return "", nil
					}
					return "", err
				}
				return configmap.Data[apply.MetalLBConfigMap], err
			}, 2*time.Second, 200*time.Millisecond).Should(MatchYAML(`address-pools:
- name: test-addresspool2
  protocol: bgp
  addresses:
  - 2.2.2.2-2.2.2.100
  auto-assign: false
`))
			By("Deleting 2nd AddressPool resource")
			err = k8sClient.Delete(context.Background(), addressPool2)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is deleted")
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: apply.MetalLBConfigMap, Namespace: MetalLBTestNameSpace}, configmap)
				if err != nil {
					// if its notfound means that was the last object and configmap is deleted
					if errors.IsNotFound(err) {
						return true
					}
				}
				return false
			}, 2*time.Second, 200*time.Millisecond).Should(BeTrue())
		})
	})
})
