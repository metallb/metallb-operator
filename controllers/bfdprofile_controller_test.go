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
	"k8s.io/utils/pointer"
)

var _ = Describe("BFD Controller", func() {
	Context("Creating BFD object", func() {
		AfterEach(func() {
			err := cleanTestNamespace()
			Expect(err).ToNot(HaveOccurred())
		})
		It("Should create BFD Objects", func() {
			profile1 := &v1alpha1.BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bfdprofile1",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: v1alpha1.BFDProfileSpec{
					ReceiveInterval:      uint32Ptr(10),
					TransmitInterval:     uint32Ptr(20),
					DetectMultiplier:     uint32Ptr(3),
					EchoReceiveInterval:  pointer.StringPtr("disabled"),
					EchoTransmitInterval: uint32Ptr(40),
					EchoMode:             pointer.BoolPtr(true),
					PassiveMode:          pointer.BoolPtr(false),
					MinimumTTL:           uint32Ptr(5),
				},
			}

			profile2 := &v1alpha1.BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bfdprofile2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: v1alpha1.BFDProfileSpec{
					ReceiveInterval:     uint32Ptr(10),
					TransmitInterval:    uint32Ptr(20),
					DetectMultiplier:    uint32Ptr(30),
					EchoReceiveInterval: pointer.StringPtr("45"),
				},
			}

			By("Creating the first BFD Profile")
			err := k8sClient.Create(context.Background(), profile1)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is created and matches the bfdprofile1 configuration")
			validateConfigMatchesYaml(`bfd-profiles:
- detect-multiplier: 3
  echo-mode: true
  echo-receive-interval: disabled
  echo-transmit-interval: 40
  minimum-ttl: 5
  name: bfdprofile1
  passive-mode: false
  receive-interval: 10
  transmit-interval: 20
`)
			By("Creating the second BFDProfile resource")
			err = k8sClient.Create(context.Background(), profile2)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is created and matches the profile1 & profile2 configuration")
			validateConfigMatchesYaml(`bfd-profiles:
- detect-multiplier: 3
  echo-mode: true
  echo-receive-interval: disabled
  echo-transmit-interval: 40
  minimum-ttl: 5
  name: bfdprofile1
  passive-mode: false
  receive-interval: 10
  transmit-interval: 20
- detect-multiplier: 30
  echo-receive-interval: "45"
  name: bfdprofile2
  receive-interval: 10
  transmit-interval: 20`)

			By("Deleting the 1st BFDProfile resource")
			err = k8sClient.Delete(context.Background(), profile1)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap matches the profile2 configuration")
			validateConfigMatchesYaml(`bfd-profiles:
- detect-multiplier: 30
  echo-receive-interval: "45"
  name: bfdprofile2
  receive-interval: 10
  transmit-interval: 20
`)
			By("Deleting 2nd BFD Profile resource")
			err = k8sClient.Delete(context.Background(), profile2)
			Expect(err).ToNot(HaveOccurred())

			By("Checking the ConfigMap is cleared")
			validateConfigMatchesYaml("{}")
		})
	})

	Context("Creating invalid BFDProfiles", func() {
		AfterEach(func() {
			err := cleanTestNamespace()
			Expect(err).ToNot(HaveOccurred())
		})
		By("Creating new profile with invalid detect multiplier value (over maximum limit)")
		badProfile1 := &v1alpha1.BFDProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "badProfile1",
				Namespace: MetalLBTestNameSpace,
			},
			Spec: v1alpha1.BFDProfileSpec{
				DetectMultiplier: uint32Ptr(999999),
			},
		}
		It("Should fail the validation", func() {
			err := k8sClient.Create(context.Background(), badProfile1)
			Expect(err).To(HaveOccurred())
		})
		By("Creating new profile with invalid receive interval value (under minimum limit)")
		badProfile2 := &v1alpha1.BFDProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "badProfile2",
				Namespace: MetalLBTestNameSpace,
			},
			Spec: v1alpha1.BFDProfileSpec{
				ReceiveInterval: uint32Ptr(1),
			},
		}
		It("Should fail the validation", func() {
			err := k8sClient.Create(context.Background(), badProfile2)
			Expect(err).To(HaveOccurred())
		})
		By("Creating new profile with invalid EchoReceiveInterval string")
		badProfile3 := &v1alpha1.BFDProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "badProfile3",
				Namespace: MetalLBTestNameSpace,
			},
			Spec: v1alpha1.BFDProfileSpec{
				EchoReceiveInterval: pointer.StringPtr("bad"),
			},
		}
		It("Should fail the validation", func() {
			err := k8sClient.Create(context.Background(), badProfile3)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Creating Full BGP + BFD configuration", func() {
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
							AggregationLength: pointer.Int32Ptr(24),
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

			peer1 := &v1alpha1.BGPPeer{
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
			peer2 := &v1alpha1.BGPPeer{
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
			profile1 := &v1alpha1.BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bfdprofile1",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: v1alpha1.BFDProfileSpec{
					ReceiveInterval:      uint32Ptr(10),
					TransmitInterval:     uint32Ptr(20),
					DetectMultiplier:     uint32Ptr(30),
					EchoReceiveInterval:  pointer.StringPtr("disabled"),
					EchoTransmitInterval: uint32Ptr(40),
					EchoMode:             pointer.BoolPtr(true),
					PassiveMode:          pointer.BoolPtr(false),
					MinimumTTL:           uint32Ptr(50),
				},
			}

			profile2 := &v1alpha1.BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bfdprofile2",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: v1alpha1.BFDProfileSpec{
					ReceiveInterval:     uint32Ptr(10),
					TransmitInterval:    uint32Ptr(20),
					DetectMultiplier:    uint32Ptr(30),
					EchoReceiveInterval: pointer.StringPtr("45"),
				},
			}

			By("Creating the first AddressPool resource")
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
    localpref: 100
`)
			By("Creating the first BGPPeer resource")
			err = k8sClient.Create(context.Background(), peer1)
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
			err = k8sClient.Create(context.Background(), peer2)
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

			By("Creating the first bfd profile resource")
			err = k8sClient.Create(context.Background(), profile1)
			Expect(err).ToNot(HaveOccurred())
			By("Checking ConfigMap is created and matches test-addresspool1,2 and bgp-peer1,2 configuration")
			validateConfigMatchesYaml(`address-pools:
- addresses:
  - 1.1.1.1-1.1.1.100
  auto-assign: false
  bgp-advertisements:
  - aggregation-length: 24
    communities:
    - 65535:65282
    - 7003:007
    localpref: 100
  name: test-addresspool1
  protocol: bgp
- addresses:
  - 2.2.2.2-2.2.2.100
  auto-assign: false
  name: test-addresspool2
  protocol: bgp
bfd-profiles:
- detect-multiplier: 30
  echo-mode: true
  echo-receive-interval: disabled
  echo-transmit-interval: 40
  minimum-ttl: 50
  name: bfdprofile1
  passive-mode: false
  receive-interval: 10
  transmit-interval: 20
peers:
- my-asn: 64500
  peer-address: 10.0.0.1
  peer-asn: 64501
  router-id: 10.10.10.10
- my-asn: 64000
  peer-address: 11.0.0.1
  peer-asn: 64001
  router-id: 11.11.11.11`)

			By("Creating the second bfd profile resource")
			err = k8sClient.Create(context.Background(), profile2)
			Expect(err).ToNot(HaveOccurred())
			By("Checking ConfigMap is created and matches test-addresspool1,2 and bgp-peer1,2 configuration")
			validateConfigMatchesYaml(`address-pools:
- addresses:
  - 1.1.1.1-1.1.1.100
  auto-assign: false
  bgp-advertisements:
  - aggregation-length: 24
    communities:
    - 65535:65282
    - 7003:007
    localpref: 100
  name: test-addresspool1
  protocol: bgp
- addresses:
  - 2.2.2.2-2.2.2.100
  auto-assign: false
  name: test-addresspool2
  protocol: bgp
bfd-profiles:
- detect-multiplier: 30
  echo-mode: true
  echo-receive-interval: disabled
  echo-transmit-interval: 40
  minimum-ttl: 50
  name: bfdprofile1
  passive-mode: false
  receive-interval: 10
  transmit-interval: 20
- detect-multiplier: 30
  echo-receive-interval: "45"
  name: bfdprofile2
  receive-interval: 10
  transmit-interval: 20
peers:
- my-asn: 64500
  peer-address: 10.0.0.1
  peer-asn: 64501
  router-id: 10.10.10.10
- my-asn: 64000
  peer-address: 11.0.0.1
  peer-asn: 64001
  router-id: 11.11.11.11`)

			By("Deleting 1st BGPPeer resource")
			err = k8sClient.Delete(context.Background(), peer1)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap matches configuration")
			validateConfigMatchesYaml(`address-pools:
- addresses:
  - 1.1.1.1-1.1.1.100
  auto-assign: false
  bgp-advertisements:
  - aggregation-length: 24
    communities:
    - 65535:65282
    - 7003:007
    localpref: 100
  name: test-addresspool1
  protocol: bgp
- addresses:
  - 2.2.2.2-2.2.2.100
  auto-assign: false
  name: test-addresspool2
  protocol: bgp
bfd-profiles:
- detect-multiplier: 30
  echo-mode: true
  echo-receive-interval: disabled
  echo-transmit-interval: 40
  minimum-ttl: 50
  name: bfdprofile1
  passive-mode: false
  receive-interval: 10
  transmit-interval: 20
- detect-multiplier: 30
  echo-receive-interval: "45"
  name: bfdprofile2
  receive-interval: 10
  transmit-interval: 20
peers:
- my-asn: 64000
  peer-address: 11.0.0.1
  peer-asn: 64001
  router-id: 11.11.11.11`)
			By("Deleting 1st AddressPool resource")
			err = k8sClient.Delete(context.Background(), addressPool1)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is created and matches test-addresspool2 and bgp-peer2 configuration")
			validateConfigMatchesYaml(`address-pools:
- addresses:
  - 2.2.2.2-2.2.2.100
  auto-assign: false
  name: test-addresspool2
  protocol: bgp
bfd-profiles:
- detect-multiplier: 30
  echo-mode: true
  echo-receive-interval: disabled
  echo-transmit-interval: 40
  minimum-ttl: 50
  name: bfdprofile1
  passive-mode: false
  receive-interval: 10
  transmit-interval: 20
- detect-multiplier: 30
  echo-receive-interval: "45"
  name: bfdprofile2
  receive-interval: 10
  transmit-interval: 20
peers:
- my-asn: 64000
  peer-address: 11.0.0.1
  peer-asn: 64001
  router-id: 11.11.11.11`)
			By("Deleting 2nd BGPPeer resource")
			err = k8sClient.Delete(context.Background(), peer2)
			Expect(err).ToNot(HaveOccurred())

			By("Checking all peers configuration is deleted and test-addresspool2 is still there")
			validateConfigMatchesYaml(`address-pools:
- addresses:
  - 2.2.2.2-2.2.2.100
  auto-assign: false
  name: test-addresspool2
  protocol: bgp
bfd-profiles:
- detect-multiplier: 30
  echo-mode: true
  echo-receive-interval: disabled
  echo-transmit-interval: 40
  minimum-ttl: 50
  name: bfdprofile1
  passive-mode: false
  receive-interval: 10
  transmit-interval: 20
- detect-multiplier: 30
  echo-receive-interval: "45"
  name: bfdprofile2
  receive-interval: 10
  transmit-interval: 20`)
			By("Deleting the remaining resources")
			err = k8sClient.Delete(context.Background(), addressPool2)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(context.Background(), profile1)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(context.Background(), profile2)
			Expect(err).ToNot(HaveOccurred())

			By("Checking ConfigMap is cleared")
			validateConfigMatchesYaml("{}")
		})
	})
})

func uint32Ptr(n uint32) *uint32 {
	return &n
}

func validateConfigMatchesYaml(toMatch string) {
	configmap := &corev1.ConfigMap{}
	EventuallyWithOffset(1, func() (string, error) {
		err := k8sClient.Get(context.Background(),
			types.NamespacedName{Name: apply.MetalLBConfigMap, Namespace: MetalLBTestNameSpace}, configmap)
		if err != nil {
			if errors.IsNotFound(err) {
				return "", nil
			}
			return "", err
		}
		return configmap.Data[apply.MetalLBConfigMap], err
	}, 2*time.Second, 200*time.Millisecond).Should(MatchYAML(toMatch))
}
