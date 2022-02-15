package controllers

import (
	"context"

	"k8s.io/utils/pointer"

	"github.com/metallb/metallb-operator/api/v1beta1"
	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Peer Controller", func() {
	Context("Creating Peer object", func() {
		BeforeEach(func() {
			metallb := &metallbv1beta1.MetalLB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "metallb",
					Namespace: MetalLBTestNameSpace,
				},
			}
			By("Creating a MetalLB resource")
			err := k8sClient.Create(context.Background(), metallb)
			Expect(err).ToNot(HaveOccurred())
		})
		AfterEach(func() {
			err := cleanTestNamespace()
			Expect(err).ToNot(HaveOccurred())
		})
		It("Should create Peer Objects", func() {
			Peer1 := &v1beta1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bgp-peer1",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: v1beta1.BGPPeerSpec{
					Address:  "10.0.0.1",
					ASN:      64501,
					MyASN:    64500,
					RouterID: "10.10.10.10",
					NodeSelectors: []v1beta1.NodeSelector{
						{
							MatchExpressions: []v1beta1.MatchExpression{
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

			Peer2 := &v1beta1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bgp-peer2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: v1beta1.BGPPeerSpec{
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
			validateConfigMatchesYaml(`peers:
- my-asn: 64500
  node-selectors:
  - match-expressions:
    - key: kubernetes.io/hostname
      operator: In
      values:
      - hostA
      - hostB
  peer-address: 10.0.0.1
  peer-asn: 64501
  router-id: 10.10.10.10
`)
			By("Creating 2nd BGPPeer resource")
			err = k8sClient.Create(context.Background(), Peer2)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is created and matches bgp-peer1 & 2 configuration")
			validateConfigMatchesYaml(`peers:
- my-asn: 64500
  node-selectors:
  - match-expressions:
    - key: kubernetes.io/hostname
      operator: In
      values:
      - hostA
      - hostB
  peer-address: 10.0.0.1
  peer-asn: 64501
  router-id: 10.10.10.10
- my-asn: 64000
  peer-address: 11.0.0.1
  peer-asn: 64001
  router-id: 11.11.11.11
`)

			By("Deleting 1st BGPPeer resource")
			err = k8sClient.Delete(context.Background(), Peer1)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap matches bgp-peer2 configuration")
			validateConfigMatchesYaml(`peers:
- my-asn: 64000
  peer-address: 11.0.0.1
  peer-asn: 64001
  router-id: 11.11.11.11
`)
			By("Deleting 2nd BGPPeer resource")
			err = k8sClient.Delete(context.Background(), Peer2)
			Expect(err).ToNot(HaveOccurred())

			By("Checking the ConfigMap is cleared")
			validateConfigMatchesYaml("{}")
		})
	})

	Context("Creating Full BGP configuration", func() {
		BeforeEach(func() {
			metallb := &metallbv1beta1.MetalLB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "metallb",
					Namespace: MetalLBTestNameSpace,
				},
			}
			By("Creating a MetalLB resource")
			err := k8sClient.Create(context.Background(), metallb)
			Expect(err).ToNot(HaveOccurred())
		})
		AfterEach(func() {
			err := cleanTestNamespace()
			Expect(err).ToNot(HaveOccurred())
		})
		It("Should create BGP Configuration Objects", func() {
			autoAssign := false
			addressPool1 := &v1beta1.AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool1",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: v1beta1.AddressPoolSpec{
					Protocol: "bgp",
					Addresses: []string{
						"1.1.1.1-1.1.1.100",
					},
					AutoAssign: &autoAssign,
					BGPAdvertisements: []v1beta1.BgpAdvertisement{
						{
							AggregationLength:   pointer.Int32Ptr(24),
							AggregationLengthV6: pointer.Int32Ptr(128),
							LocalPref:           100,
							Communities: []string{
								"65535:65282",
								"7003:007",
							},
						},
					},
				},
			}
			addressPool2 := &v1beta1.AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: v1beta1.AddressPoolSpec{
					Protocol: "bgp",
					Addresses: []string{
						"2.2.2.2-2.2.2.100",
					},
					AutoAssign: &autoAssign,
				},
			}

			Peer1 := &v1beta1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bgp-peer1",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: v1beta1.BGPPeerSpec{
					Address:  "10.0.0.1",
					ASN:      64501,
					MyASN:    64500,
					RouterID: "10.10.10.10",
				},
			}
			Peer2 := &v1beta1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bgp-peer2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: v1beta1.BGPPeerSpec{
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
			validateConfigMatchesYaml(`address-pools:
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
    aggregation-length-v6: 128
    localpref: 100
`)
			By("Creating 1st BGPPeer resource")
			err = k8sClient.Create(context.Background(), Peer1)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is created and matches test-addresspool1 and bgp-peer1 configuration")
			validateConfigMatchesYaml(`address-pools:
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
    aggregation-length-v6: 128
    localpref: 100
peers:
- my-asn: 64500
  peer-address: 10.0.0.1
  peer-asn: 64501
  router-id: 10.10.10.10
`)
			By("Creating 2nd AddressPool resource")
			err = k8sClient.Create(context.Background(), addressPool2)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is created and matches test-addresspool1,2 and bgp-peer1 configuration")
			validateConfigMatchesYaml(`address-pools:
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
    aggregation-length-v6: 128
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
`)
			By("Creating 2nd BGPPeer resource")
			err = k8sClient.Create(context.Background(), Peer2)
			Expect(err).ToNot(HaveOccurred())
			By("Checking ConfigMap is created and matches test-addresspool1,2 and bgp-peer1,2 configuration")
			validateConfigMatchesYaml(`address-pools:
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
    aggregation-length-v6: 128
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
`)
			By("Deleting 1st BGPPeer resource")
			err = k8sClient.Delete(context.Background(), Peer1)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap matches configuration")
			validateConfigMatchesYaml(`address-pools:
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
    aggregation-length-v6: 128
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
`)
			By("Deleting 1st AddressPool resource")
			err = k8sClient.Delete(context.Background(), addressPool1)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is created and matches test-addresspool2 and bgp-peer2 configuration")
			validateConfigMatchesYaml(`address-pools:
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
`)
			By("Deleting 2nd BGPPeer resource")
			err = k8sClient.Delete(context.Background(), Peer2)
			Expect(err).ToNot(HaveOccurred())

			By("Checking all peers configuration is deleted and test-addresspool2 is still there")
			validateConfigMatchesYaml(`address-pools:
- name: test-addresspool2
  protocol: bgp
  addresses:
  - 2.2.2.2-2.2.2.100
  auto-assign: false
`)
			By("Deleting 2nd AddressPool resource")
			err = k8sClient.Delete(context.Background(), addressPool2)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is cleared")
			validateConfigMatchesYaml("{}")
		})
	})
})
