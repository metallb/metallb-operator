package tests

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"k8s.io/utils/pointer"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	metallbv1alpha1 "github.com/metallb/metallb-operator/api/v1alpha1"
	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/pkg/status"
	"github.com/metallb/metallb-operator/test/consts"
	testclient "github.com/metallb/metallb-operator/test/e2e/client"
	metallbutils "github.com/metallb/metallb-operator/test/e2e/metallb"
	admv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	goclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var autoAssign = false
var UseMetallbResourcesFromFile = false

var OperatorNameSpace = consts.DefaultOperatorNameSpace

func init() {
	if len(os.Getenv("USE_LOCAL_RESOURCES")) != 0 {
		UseMetallbResourcesFromFile = true
	}

	if ns := os.Getenv("OO_INSTALL_NAMESPACE"); len(ns) != 0 {
		OperatorNameSpace = ns
	}
}

var _ = Describe("metallb", func() {
	Context("MetalLB deploy", func() {
		var metallb *metallbv1beta1.MetalLB
		var metallbCRExisted bool

		BeforeEach(func() {
			var err error
			metallb, err = metallbutils.Get(OperatorNameSpace, UseMetallbResourcesFromFile)
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
				deployment, err := testclient.Client.Deployments(metallb.Namespace).Get(context.Background(), consts.MetalLBDeploymentName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(deployment.OwnerReferences).ToNot(BeNil())
				Expect(deployment.OwnerReferences[0].Kind).To(Equal("MetalLB"))

				daemonset, err := testclient.Client.DaemonSets(metallb.Namespace).Get(context.Background(), consts.MetalLBDaemonsetName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(daemonset.OwnerReferences).ToNot(BeNil())
				Expect(daemonset.OwnerReferences[0].Kind).To(Equal("MetalLB"))

				metallbutils.Delete(metallb)
			}
		})

		It("should have MetalLB pods in running state", func() {
			By("checking MetalLB controller deployment is in running state", func() {
				Eventually(func() bool {
					deploy, err := testclient.Client.Deployments(metallb.Namespace).Get(context.Background(), consts.MetalLBDeploymentName, metav1.GetOptions{})
					if err != nil {
						return false
					}
					return deploy.Status.ReadyReplicas > 0 && deploy.Status.ReadyReplicas == deploy.Status.Replicas
				}, metallbutils.DeployTimeout, metallbutils.Interval).Should(BeTrue())

				pods, err := testclient.Client.Pods(OperatorNameSpace).List(context.Background(), metav1.ListOptions{
					LabelSelector: "component=controller"})
				Expect(err).ToNot(HaveOccurred())

				deploy, err := testclient.Client.Deployments(metallb.Namespace).Get(context.Background(), consts.MetalLBDeploymentName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(len(pods.Items)).To(Equal(int(deploy.Status.Replicas)))

				for _, pod := range pods.Items {
					Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
				}
			})

			By("checking MetalLB daemonset is in running state", func() {
				Eventually(func() bool {
					daemonset, err := testclient.Client.DaemonSets(metallb.Namespace).Get(context.Background(), consts.MetalLBDaemonsetName, metav1.GetOptions{})
					if err != nil {
						return false
					}
					return daemonset.Status.DesiredNumberScheduled == daemonset.Status.NumberReady
				}, metallbutils.DeployTimeout, metallbutils.Interval).Should(BeTrue())

				pods, err := testclient.Client.Pods(OperatorNameSpace).List(context.Background(), metav1.ListOptions{
					LabelSelector: "component=speaker"})
				Expect(err).ToNot(HaveOccurred())

				daemonset, err := testclient.Client.DaemonSets(metallb.Namespace).Get(context.Background(), consts.MetalLBDaemonsetName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(len(pods.Items)).To(Equal(int(daemonset.Status.DesiredNumberScheduled)))

				for _, pod := range pods.Items {
					Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
				}
			})
			By("checking MetalLB CR status is set", func() {
				Eventually(func() bool {
					config := &metallbv1beta1.MetalLB{}
					err := testclient.Client.Get(context.Background(), goclient.ObjectKey{Namespace: metallb.Namespace, Name: metallb.Name}, config)
					Expect(err).ToNot(HaveOccurred())
					if config.Status.Conditions == nil {
						return false
					}
					for _, condition := range config.Status.Conditions {
						switch condition.Type {
						case status.ConditionAvailable:
							if condition.Status == metav1.ConditionFalse {
								return false
							}
						case status.ConditionProgressing:
							if condition.Status == metav1.ConditionTrue {
								return false
							}
						case status.ConditionDegraded:
							if condition.Status == metav1.ConditionTrue {
								return false
							}
						case status.ConditionUpgradeable:
							if condition.Status == metav1.ConditionFalse {
								return false
							}
						}
					}
					return true
				}, 5*time.Minute, 5*time.Second).Should(BeTrue())
			})
		})
	})

	Context("Creating AddressPool", func() {
		table.DescribeTable("Testing creating addresspool CR successfully", func(addressPoolName string, addresspool client.Object, expectedConfigMap string) {
			By("Creating AddressPool CR")

			Expect(testclient.Client.Create(context.Background(), addresspool)).Should(Succeed())

			key := types.NamespacedName{
				Name:      addressPoolName,
				Namespace: OperatorNameSpace,
			}
			// Create addresspool resource
			By("Checking AddressPool resource is created")
			Eventually(func() error {
				err := testclient.Client.Get(context.Background(), key, addresspool)
				return err
			}, metallbutils.Timeout, metallbutils.Interval).Should(Succeed())

			By("Checking ConfigMap is created match the expected configuration")
			Eventually(func() (string, error) {
				configmap, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
				if err != nil {
					return "", err
				}
				return configmap.Data[consts.MetalLBConfigMapName], err
			}, metallbutils.Timeout, metallbutils.Interval).Should(MatchYAML(expectedConfigMap))

			By("Checking AddressPool resource is deleted and ConfigMap is cleared")
			err := testclient.Client.Delete(context.Background(), addresspool)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() string {
				configmap, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return configmap.Data[consts.MetalLBConfigMapName]
			}, metallbutils.Timeout, metallbutils.Interval).Should(MatchYAML("{}"))
		},
			table.Entry("Test AddressPool object with default auto assign", "addresspool1", &metallbv1beta1.AddressPool{
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
			}, `address-pools:
- name: addresspool1
  protocol: layer2
  addresses:
  - 1.1.1.1-1.1.1.100
`),
			table.Entry("Test AddressPool object v1alpha1", "addresspool1", &metallbv1alpha1.AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "addresspool1",
					Namespace: OperatorNameSpace,
				},
				Spec: metallbv1alpha1.AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"1.1.1.1-1.1.1.100",
					},
				},
			}, `address-pools:
- name: addresspool1
  protocol: layer2
  addresses:
  - 1.1.1.1-1.1.1.100
`),
			table.Entry("Test AddressPool object with auto assign set to false", "addresspool2", &metallbv1beta1.AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "addresspool2",
					Namespace: OperatorNameSpace,
				},
				Spec: metallbv1beta1.AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"2.2.2.1-2.2.2.100",
					},
					AutoAssign: &autoAssign,
				},
			}, `address-pools:
- name: addresspool2
  protocol: layer2
  auto-assign: false
  addresses:
  - 2.2.2.1-2.2.2.100
`),
			table.Entry("Test AddressPool object with bgp-advertisements", "addresspool3", &metallbv1beta1.AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "addresspool3",
					Namespace: OperatorNameSpace,
				},
				Spec: metallbv1beta1.AddressPoolSpec{
					Protocol: "bgp",
					Addresses: []string{
						"3.3.3.1-3.3.3.100",
					},
					AutoAssign: &autoAssign,
					BGPAdvertisements: []metallbv1beta1.BgpAdvertisement{
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
			}, `address-pools:
- name: addresspool3
  protocol: bgp
  auto-assign: false
  addresses:
  - 3.3.3.1-3.3.3.100
  bgp-advertisements: 
  - communities: 
    - 65535:65282
    - 7003:007
    aggregation-length: 24
    aggregation-length-v6: 124
    localpref: 100
`),
			table.Entry("Test AddressPool object with bgp-advertisements", "addresspool4", &metallbv1beta1.AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "addresspool4",
					Namespace: OperatorNameSpace,
				},
				Spec: metallbv1beta1.AddressPoolSpec{
					Protocol: "bgp",
					Addresses: []string{
						"4.4.4.1-4.4.4.100",
					},
					AutoAssign: &autoAssign,
					BGPAdvertisements: []metallbv1beta1.BgpAdvertisement{
						{
							LocalPref: 100,
							Communities: []string{
								"65535:65282",
								"7003:007",
							},
						},
					},
				},
			}, `address-pools:
- name: addresspool4
  protocol: bgp
  auto-assign: false
  addresses:
  - 4.4.4.1-4.4.4.100
  bgp-advertisements: 
  - communities: 
    - 65535:65282
    - 7003:007
    aggregation-length: 32
    aggregation-length-v6: 128
    localpref: 100
`),
		)
	})
	Context("MetalLB contains incorrect data", func() {
		Context("MetalLB has incorrect name", func() {

			var metallb *metallbv1beta1.MetalLB
			BeforeEach(func() {
				var err error
				metallb, err = metallbutils.Get(OperatorNameSpace, UseMetallbResourcesFromFile)
				Expect(err).ToNot(HaveOccurred())
				metallb.SetName("incorrectname")
				Expect(testclient.Client.Create(context.Background(), metallb)).Should(Succeed())
			})

			AfterEach(func() {
				metallbutils.Delete(metallb)
			})
			It("should not be reconciled", func() {
				By("checking MetalLB resource status", func() {
					Eventually(func() bool {
						instance := &metallbv1beta1.MetalLB{}
						err := testclient.Client.Get(context.TODO(), goclient.ObjectKey{Namespace: metallb.Namespace, Name: metallb.Name}, instance)
						Expect(err).ToNot(HaveOccurred())
						for _, condition := range instance.Status.Conditions {
							if condition.Type == status.ConditionDegraded && condition.Status == metav1.ConditionTrue {
								return true
							}
						}
						return false
					}, 30*time.Second, 5*time.Second).Should(BeTrue())
				})
			})
		})

		Context("Correct and incorrect MetalLB resources coexist", func() {
			var correct_metallb *metallbv1beta1.MetalLB
			var incorrect_metallb *metallbv1beta1.MetalLB
			BeforeEach(func() {
				var err error
				correct_metallb, err = metallbutils.Get(OperatorNameSpace, UseMetallbResourcesFromFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(testclient.Client.Create(context.Background(), correct_metallb)).Should(Succeed())

				incorrect_metallb, err = metallbutils.Get(OperatorNameSpace, UseMetallbResourcesFromFile)
				Expect(err).ToNot(HaveOccurred())
				incorrect_metallb.SetName("incorrectname")
				Expect(testclient.Client.Create(context.Background(), incorrect_metallb)).Should(Succeed())
			})

			AfterEach(func() {
				metallbutils.Delete(incorrect_metallb)
				metallbutils.Delete(correct_metallb)
			})
			It("should have correct statuses", func() {
				By("checking MetalLB resource status", func() {
					Eventually(func() bool {
						instance := &metallbv1beta1.MetalLB{}
						err := testclient.Client.Get(context.TODO(), goclient.ObjectKey{Namespace: incorrect_metallb.Namespace, Name: incorrect_metallb.Name}, instance)
						Expect(err).ToNot(HaveOccurred())
						return metallbutils.CheckConditionStatus(instance) == status.ConditionDegraded
					}, 30*time.Second, 5*time.Second).Should(BeTrue())

					Eventually(func() bool {
						instance := &metallbv1beta1.MetalLB{}
						err := testclient.Client.Get(context.TODO(), goclient.ObjectKey{Namespace: correct_metallb.Namespace, Name: correct_metallb.Name}, instance)
						Expect(err).ToNot(HaveOccurred())
						return metallbutils.CheckConditionStatus(instance) == status.ConditionAvailable
					}, 30*time.Second, 5*time.Second).Should(BeTrue())

					// Delete incorrectly named resource
					err := testclient.Client.Delete(context.Background(), incorrect_metallb)
					Expect(err).ToNot(HaveOccurred())
					Eventually(func() bool {
						err := testclient.Client.Get(context.Background(), goclient.ObjectKey{Namespace: incorrect_metallb.Namespace, Name: incorrect_metallb.Name}, incorrect_metallb)
						return errors.IsNotFound(err)
					}, 1*time.Minute, 5*time.Second).Should(BeTrue(), "Failed to delete MetalLB custom resource")

					// Correctly named resource status should not change
					Eventually(func() bool {
						instance := &metallbv1beta1.MetalLB{}
						err := testclient.Client.Get(context.TODO(), goclient.ObjectKey{Namespace: correct_metallb.Namespace, Name: correct_metallb.Name}, instance)
						Expect(err).ToNot(HaveOccurred())
						return metallbutils.CheckConditionStatus(instance) == status.ConditionAvailable
					}, 30*time.Second, 5*time.Second).Should(BeTrue())
				})
			})
		})
	})
	Context("Testing create/delete Multiple AddressPools", func() {
		It("should have created, merged and deleted resources correctly", func() {
			By("Creating first addresspool object ", func() {
				addresspool := &metallbv1beta1.AddressPool{
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
				}

				Expect(testclient.Client.Create(context.Background(), addresspool)).Should(Succeed())

				key := types.NamespacedName{
					Name:      "addresspool1",
					Namespace: OperatorNameSpace,
				}
				By("Checking AddressPool1 resource is created")
				Eventually(func() error {
					err := testclient.Client.Get(context.Background(), key, addresspool)
					return err
				}, metallbutils.Timeout, metallbutils.Interval).Should(Succeed())

				By("Checking ConfigMap is created and matches addresspool1 configuration")
				Eventually(func() (string, error) {
					configmap, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
					if err != nil {
						return "", err
					}
					return configmap.Data[consts.MetalLBConfigMapName], err
				}, metallbutils.Timeout, metallbutils.Interval).Should(MatchYAML(`address-pools:
- name: addresspool1
  protocol: layer2
  addresses:
  - 1.1.1.1-1.1.1.100
`))

			})

			By("Creating second addresspool object ", func() {
				addresspool := &metallbv1beta1.AddressPool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "addresspool2",
						Namespace: OperatorNameSpace,
					},
					Spec: metallbv1beta1.AddressPoolSpec{
						Protocol: "layer2",
						Addresses: []string{
							"2.2.2.1-2.2.2.100",
						},
						AutoAssign: &autoAssign,
					},
				}

				Expect(testclient.Client.Create(context.Background(), addresspool)).Should(Succeed())

				key := types.NamespacedName{
					Name:      "addresspool2",
					Namespace: OperatorNameSpace,
				}
				By("Checking AddressPool2 resource is created")
				Eventually(func() error {
					err := testclient.Client.Get(context.Background(), key, addresspool)
					return err
				}, metallbutils.Timeout, metallbutils.Interval).Should(Succeed())

				By("Checking ConfigMap is created and matches addresspool2 configuration")
				Eventually(func() (string, error) {
					configmap, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
					if err != nil {
						return "", err
					}
					return configmap.Data[consts.MetalLBConfigMapName], err
				}, metallbutils.Timeout, metallbutils.Interval).Should(MatchYAML(`address-pools:
- name: addresspool1
  protocol: layer2
  addresses:
  - 1.1.1.1-1.1.1.100
- name: addresspool2
  protocol: layer2
  auto-assign: false
  addresses:
  - 2.2.2.1-2.2.2.100
`))
			})

			By("Deleting the first addresspool object", func() {
				addresspool := &metallbv1beta1.AddressPool{
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
				}
				err := testclient.Client.Delete(context.Background(), addresspool)
				Expect(err).ToNot(HaveOccurred())

				By("Checking ConfigMap matches the expected configuration")
				Eventually(func() (string, error) {
					configmap, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
					if err != nil {
						// if its notfound means that was the last addresspool and configmap is deleted
						if errors.IsNotFound(err) {
							return "", nil
						}
						return "", err
					}
					return configmap.Data[consts.MetalLBConfigMapName], err
				}, metallbutils.Timeout, metallbutils.Interval).Should(MatchYAML(`address-pools:
- name: addresspool2
  protocol: layer2
  auto-assign: false
  addresses:
  - 2.2.2.1-2.2.2.100
`))

			})

			By("Deleting the second addresspool object", func() {
				addresspool := &metallbv1beta1.AddressPool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "addresspool2",
						Namespace: OperatorNameSpace,
					},
					Spec: metallbv1beta1.AddressPoolSpec{
						Protocol: "layer2",
						Addresses: []string{
							"2.2.2.1-2.2.2.100",
						},
					},
				}

				err := testclient.Client.Delete(context.Background(), addresspool)
				Expect(err).ToNot(HaveOccurred())
			})

			By("Checking ConfigMap is cleared at the end of the test")
			Eventually(func() string {
				configmap, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return configmap.Data[consts.MetalLBConfigMapName]
			}, metallbutils.Timeout, metallbutils.Interval).Should(MatchYAML("{}"))
		})
	})

	Context("Testing Update AddressPool", func() {
		It("should have created, update and finally delete addresspool correctly", func() {
			By("Creating addresspool object ", func() {
				addresspool := &metallbv1beta1.AddressPool{
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
				}

				Expect(testclient.Client.Create(context.Background(), addresspool)).Should(Succeed())

				key := types.NamespacedName{
					Name:      "addresspool1",
					Namespace: OperatorNameSpace,
				}
				By("Checking AddressPool resource is created")
				Eventually(func() error {
					err := testclient.Client.Get(context.Background(), key, addresspool)
					return err
				}, metallbutils.Timeout, metallbutils.Interval).Should(Succeed())

				By("Checking ConfigMap is created and matches addresspool configuration")
				Eventually(func() (string, error) {
					configmap, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
					if err != nil {
						return "", err
					}
					return configmap.Data[consts.MetalLBConfigMapName], err
				}, metallbutils.Timeout, metallbutils.Interval).Should(MatchYAML(`address-pools:
- name: addresspool1
  protocol: layer2
  addresses:
  - 1.1.1.1-1.1.1.100
`))

			})

			By("Update the same addresspool object with different range ", func() {
				addresspool := &metallbv1beta1.AddressPool{}
				key := types.NamespacedName{
					Name:      "addresspool1",
					Namespace: OperatorNameSpace,
				}
				Eventually(func() error {
					err := testclient.Client.Get(context.Background(), key, addresspool)
					return err
				}, metallbutils.Timeout, metallbutils.Interval).Should(Succeed())

				addresspool.Spec = metallbv1beta1.AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"1.1.1.1-1.1.1.200",
					},
					AutoAssign: &autoAssign,
				}

				Eventually(func() error {
					err := testclient.Client.Update(context.Background(), addresspool)
					return err
				}, metallbutils.Timeout, metallbutils.Interval).Should(Succeed())

				By("Checking ConfigMap is created and matches updated configuration")
				Eventually(func() (string, error) {
					configmap, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
					if err != nil {
						return "", err
					}
					return configmap.Data[consts.MetalLBConfigMapName], err
				}, metallbutils.Timeout, metallbutils.Interval).Should(MatchYAML(`address-pools:
- name: addresspool1
  protocol: layer2
  auto-assign: false
  addresses:
  - 1.1.1.1-1.1.1.200
`))
			})

			By("Deleting the addresspool object", func() {
				addresspool := &metallbv1beta1.AddressPool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "addresspool1",
						Namespace: OperatorNameSpace,
					},
					Spec: metallbv1beta1.AddressPoolSpec{
						Protocol: "layer2",
						Addresses: []string{
							"1.1.1.1-1.1.1.200",
						},
					},
				}
				err := testclient.Client.Delete(context.Background(), addresspool)
				Expect(err).ToNot(HaveOccurred())
			})

			By("Checking ConfigMap is cleared at the end of the test")
			// Make sure Configmap is cleared at the end of this test
			Eventually(func() string {
				configmap, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return configmap.Data[consts.MetalLBConfigMapName]
			}, metallbutils.Timeout, metallbutils.Interval).Should(MatchYAML("{}"))
		})
	})

	Context("Creating BGP Peer", func() {
		table.DescribeTable("Testing creating BGP peer CR successfully", func(peerName string, peer *metallbv1alpha1.BGPPeer, expectedConfigMap string) {
			By("Creating BGP peer CR")

			Expect(testclient.Client.Create(context.Background(), peer)).Should(Succeed())

			key := types.NamespacedName{
				Name:      peerName,
				Namespace: OperatorNameSpace,
			}
			// Create BGP Peer resource
			By("Checking BGP peer resource is created")
			Eventually(func() error {
				err := testclient.Client.Get(context.Background(), key, peer)
				return err
			}, metallbutils.Timeout, metallbutils.Interval).Should(Succeed())

			By("Checking ConfigMap is created match the expected configuration")
			Eventually(func() (string, error) {
				configmap, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
				if err != nil {
					return "", err
				}
				return configmap.Data[consts.MetalLBConfigMapName], err
			}, metallbutils.Timeout, metallbutils.Interval).Should(MatchYAML(expectedConfigMap))

			By("Checking the ConfigMap is cleared")
			err := testclient.Client.Delete(context.Background(), peer)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() string {
				configmap, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return configmap.Data[consts.MetalLBConfigMapName]
			}, metallbutils.Timeout, metallbutils.Interval).Should(MatchYAML("{}"))
		},
			table.Entry("Test BGP Peer object", "peer1", &metallbv1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "peer1",
					Namespace: OperatorNameSpace,
				},
				Spec: metallbv1alpha1.BGPPeerSpec{
					Address:  "10.0.0.1",
					ASN:      64501,
					MyASN:    64500,
					RouterID: "10.10.10.10",
					NodeSelectors: []metallbv1alpha1.NodeSelector{
						{
							MatchExpressions: []metallbv1alpha1.MatchExpression{
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
			}, `peers:
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
`),
			table.Entry("Test BGP Peer object", "peer2", &metallbv1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "peer2",
					Namespace: OperatorNameSpace,
				},
				Spec: metallbv1alpha1.BGPPeerSpec{
					Address:  "11.0.0.1",
					ASN:      60501,
					MyASN:    60500,
					RouterID: "11.11.11.11",
					NodeSelectors: []metallbv1alpha1.NodeSelector{
						{
							MatchExpressions: []metallbv1alpha1.MatchExpression{
								{
									Key:      "kubernetes.io/hostname",
									Operator: "Out",
									Values: []string{
										"hostC",
									},
								},
							},
						},
					},
				},
			}, `peers:
- my-asn: 60500
  node-selectors:
  - match-expressions:
    - key: kubernetes.io/hostname
      operator: Out
      values:
      - hostC
  peer-address: 11.0.0.1
  peer-asn: 60501
  router-id: 11.11.11.11 
`))
	})

	Context("Validate AddressPool Webhook", func() {
		BeforeEach(func() {
			By("Checking if validation webhook is enabled")
			deploy, err := testclient.Client.Deployments(OperatorNameSpace).Get(context.Background(), consts.MetalLBOperatorDeploymentName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			isValidationWebhookEnabled := false
			for _, container := range deploy.Spec.Template.Spec.Containers {
				if container.Name == "manager" {
					for _, env := range container.Env {
						if env.Name == "ENABLE_OPERATOR_WEBHOOK" {
							if env.Value == "true" {
								isValidationWebhookEnabled = true
							}
						}
					}
				}
			}

			if !isValidationWebhookEnabled {
				Skip("AddressPool webhook is disabled")
			}

			By("Checking if validation webhook is running")
			// Can't just check the ValidatingWebhookConfiguration name as it's changing between different deployment methods.
			// Need to check the webhook name in the webhooks definition of the ValidatingWebhookConfiguration.
			validateCfgList := &admv1.ValidatingWebhookConfigurationList{}
			err = testclient.Client.List(context.TODO(), validateCfgList, &goclient.ListOptions{})
			Expect(err).ToNot(HaveOccurred())

			isAddresspoolValidationWebhookRunning := false
			for _, validateCfg := range validateCfgList.Items {
				for _, webhook := range validateCfg.Webhooks {
					if webhook.Name == consts.AddressPoolValidationWebhookName {
						isAddresspoolValidationWebhookRunning = true
					}
				}
			}
			Expect(isAddresspoolValidationWebhookRunning).To(BeTrue(), "AddressPool webhook is not running")
		})
		It("Should recognize overlapping addresses in two AddressPools", func() {
			By("Creating first AddressPool resource")
			autoAssign := false
			firstAddressPool := &metallbv1beta1.AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool",
					Namespace: OperatorNameSpace,
				},
				Spec: metallbv1beta1.AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"1.1.1.1-1.1.1.100",
					},
					AutoAssign: &autoAssign,
				},
			}
			err := testclient.Client.Create(context.Background(), firstAddressPool)
			Expect(err).ToNot(HaveOccurred())

			By("Creating second AddressPool resource with overlapping addresses defined by address range")
			secondAdressPool := &metallbv1beta1.AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-addresspool2",
					Namespace: OperatorNameSpace,
				},
				Spec: metallbv1beta1.AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"1.1.1.15-1.1.1.20",
					},
					AutoAssign: &autoAssign,
				},
			}

			err = testclient.Client.Create(context.Background(), secondAdressPool)
			Expect(err).ToNot(BeNil())
			if !strings.Contains(fmt.Sprint(err), "overlaps with already defined CIDR") {
				Expect(err).ToNot(HaveOccurred())
			}

			By("Creating second valid AddressPool resource")
			secondAdressPool.Spec.Addresses = []string{
				"1.1.1.101-1.1.1.200",
			}
			err = testclient.Client.Create(context.Background(), secondAdressPool)
			Expect(err).ToNot(HaveOccurred())

			By("Updating second AddressPool addresses to overlapping addresses defined by network prefix")
			secondAdressPool.Spec.Addresses = []string{
				"1.1.1.0/24",
			}
			err = testclient.Client.Update(context.Background(), secondAdressPool)
			Expect(err).ToNot(BeNil())
			if !strings.Contains(fmt.Sprint(err), "overlaps with already defined CIDR") {
				Expect(err).ToNot(HaveOccurred())
			}

			By("Deleting first AddressPool resource")
			err = testclient.Client.Delete(context.Background(), firstAddressPool)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				err := testclient.Client.Get(context.Background(), goclient.ObjectKey{Namespace: firstAddressPool.Namespace, Name: firstAddressPool.Name}, firstAddressPool)
				return errors.IsNotFound(err)
			}, 1*time.Minute, 5*time.Second).Should(BeTrue(), "Failed to delete first AddressPool resource")

			By("Deleting second AddressPool resource")
			err = testclient.Client.Delete(context.Background(), secondAdressPool)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				err := testclient.Client.Get(context.Background(), goclient.ObjectKey{Namespace: secondAdressPool.Namespace, Name: secondAdressPool.Name}, secondAdressPool)
				return errors.IsNotFound(err)
			}, 1*time.Minute, 5*time.Second).Should(BeTrue(), "Failed to delete second AddressPool resource")

		})
	})

	Context("Validate BGPPeer Webhook", func() {
		BeforeEach(func() {
			By("Checking if validation webhook is enabled")
			deploy, err := testclient.Client.Deployments(OperatorNameSpace).Get(context.Background(), consts.MetalLBOperatorDeploymentName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			isValidationWebhookEnabled := false
			for _, container := range deploy.Spec.Template.Spec.Containers {
				if container.Name == "manager" {
					for _, env := range container.Env {
						if env.Name == "ENABLE_OPERATOR_WEBHOOK" {
							if env.Value == "true" {
								isValidationWebhookEnabled = true
							}
						}
					}
				}
			}

			if !isValidationWebhookEnabled {
				Skip("BGPPeer webhook is disabled")
			}

			By("Checking if validation webhook is running")
			// Can't just check the ValidatingWebhookConfiguration name as it's changing between different deployment methods.
			// Need to check the webhook name in the webhooks definition of the ValidatingWebhookConfiguration.
			validateCfgList := &admv1.ValidatingWebhookConfigurationList{}
			err = testclient.Client.List(context.TODO(), validateCfgList, &goclient.ListOptions{})
			Expect(err).ToNot(HaveOccurred())

			isBGPPeerValidationWebhookRunning := false
			for _, validateCfg := range validateCfgList.Items {
				for _, webhook := range validateCfg.Webhooks {
					if webhook.Name == consts.BGPPeerValidationWebhookName {
						isBGPPeerValidationWebhookRunning = true
					}
				}
			}
			Expect(isBGPPeerValidationWebhookRunning).To(BeTrue(), "BGPPeer validation webhook is not running")
		})
		It("Should reject invalid BGPPeer IP address", func() {
			By("Creating BGPPeer resource")
			peer := &metallbv1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgp-peer1",
					Namespace: OperatorNameSpace,
				},
				Spec: metallbv1alpha1.BGPPeerSpec{
					Address: "1.1.1",
					ASN:     64500,
					MyASN:   1000,
				},
			}
			err := testclient.Client.Create(context.Background(), peer)
			if !strings.Contains(fmt.Sprint(err), "Invalid BGPPeer address") {
				Expect(err).ToNot(HaveOccurred())
			}

			By("Updating BGPPeer resource to use valid peer address")
			peer.Spec.Address = "1.1.1.1"
			err = testclient.Client.Create(context.Background(), peer)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting BGPPeer resource")
			err = testclient.Client.Delete(context.Background(), peer)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				err := testclient.Client.Get(context.Background(), goclient.ObjectKey{Namespace: peer.Namespace, Name: peer.Name}, peer)
				return errors.IsNotFound(err)
			}, 1*time.Minute, 5*time.Second).Should(BeTrue(), "Failed to delete BGPPeer resource")
		})
		It("Should reject invalid Keepalive time", func() {
			By("Creating BGPPeer resource")
			peer := &metallbv1alpha1.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bgp-peer1",
					Namespace: OperatorNameSpace,
				},
				Spec: metallbv1alpha1.BGPPeerSpec{
					Address:       "1.1.1.1",
					ASN:           64500,
					MyASN:         1000,
					KeepaliveTime: 180 * time.Second,
					HoldTime:      90 * time.Second,
				},
			}
			err := testclient.Client.Create(context.Background(), peer)
			if !strings.Contains(fmt.Sprint(err), "Invalid keepalive time") {
				Expect(err).ToNot(HaveOccurred())
			}

			By("Updating BGPPeer resource to use valid keepalive time")
			peer.Spec.KeepaliveTime = 90 * time.Second
			err = testclient.Client.Create(context.Background(), peer)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting BGPPeer resource")
			err = testclient.Client.Delete(context.Background(), peer)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				err := testclient.Client.Get(context.Background(), goclient.ObjectKey{Namespace: peer.Namespace, Name: peer.Name}, peer)
				return errors.IsNotFound(err)
			}, 1*time.Minute, 5*time.Second).Should(BeTrue(), "Failed to delete BGPPeer resource")
		})
	})
})
