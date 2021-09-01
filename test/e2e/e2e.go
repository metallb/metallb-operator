package e2e

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"testing"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"

	metallbv1alpha1 "github.com/metallb/metallb-operator/api/v1alpha1"
	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/pkg/platform"
	"github.com/metallb/metallb-operator/pkg/status"
	"github.com/metallb/metallb-operator/test/consts"
	testclient "github.com/metallb/metallb-operator/test/e2e/client"
	"github.com/metallb/metallb-operator/test/e2e/k8sreporter"
	corev1 "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	goclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Timeout and Interval settings
	timeout  = time.Minute * 3
	interval = time.Second * 2
)

var autoAssign = false

var TestIsOpenShift = false
var UseMetallbResourcesFromFile = false

var OperatorNameSpace = "metallb-system"

var junitPath *string
var reportPath *string

func init() {
	if len(os.Getenv("IS_OPENSHIFT")) != 0 {
		TestIsOpenShift = true
	}

	if len(os.Getenv("USE_LOCAL_RESOURCES")) != 0 {
		UseMetallbResourcesFromFile = true
	}

	if ns := os.Getenv("OO_INSTALL_NAMESPACE"); len(ns) != 0 {
		OperatorNameSpace = ns
	}

	junitPath = flag.String("junit", "", "the path for the junit format report")
	reportPath = flag.String("report", "", "the path of the report file containing details for failed tests")
}

func RunE2ETests(t *testing.T) {
	RegisterFailHandler(Fail)

	rr := []Reporter{}
	if *junitPath != "" {
		junitFile := path.Join(*junitPath, "e2e_junit.xml")
		rr = append(rr, reporters.NewJUnitReporter(junitFile))
	}

	clients := testclient.New("")

	if *reportPath != "" {
		rr = append(rr, k8sreporter.New(clients, OperatorNameSpace, *reportPath))
	}

	RunSpecsWithDefaultAndCustomReporters(t, "Metallb Operator Validation Suite", rr)
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
			}, timeout, interval).Should(BeTrue())

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
	})

	Context("MetalLB deploy", func() {
		var metallb *metallbv1beta1.MetalLB
		var metallbCRExisted bool

		BeforeEach(func() {
			var err error
			metallb, err = getMetalLB()
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

				deleteMetalLB(metallb)
			}
		})

		It("should have MetalLB pods in running state", func() {
			By("checking MetalLB controller deployment is in running state", func() {
				Eventually(func() bool {
					deploy, err := testclient.Client.Deployments(metallb.Namespace).Get(context.Background(), consts.MetalLBDeploymentName, metav1.GetOptions{})
					if err != nil {
						return false
					}
					return deploy.Status.ReadyReplicas == deploy.Status.Replicas
				}, timeout, interval).Should(BeTrue())

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
				}, timeout, interval).Should(BeTrue())

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
		table.DescribeTable("Testing creating addresspool CR successfully", func(addressPoolName string, addresspool *metallbv1alpha1.AddressPool, expectedConfigMap string) {
			By("By creating AddressPool CR")

			Expect(testclient.Client.Create(context.Background(), addresspool)).Should(Succeed())

			key := types.NamespacedName{
				Name:      addressPoolName,
				Namespace: OperatorNameSpace,
			}
			// Create addresspool resource
			By("By checking AddressPool resource is created")
			Eventually(func() error {
				err := testclient.Client.Get(context.Background(), key, addresspool)
				return err
			}, timeout, interval).Should(Succeed())

			// Checking ConfigMap is created
			By("By checking ConfigMap is created match the expected configuration")
			Eventually(func() (string, error) {
				configmap, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
				if err != nil {
					return "", err
				}
				return configmap.Data[consts.MetalLBConfigMapName], err
			}, timeout, interval).Should(MatchYAML(expectedConfigMap))

			By("By checking AddressPool resource and ConfigMap are deleted")
			Eventually(func() bool {
				err := testclient.Client.Delete(context.Background(), addresspool)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue(), "Failed to delete AddressPool custom resource")

			Eventually(func() bool {
				_, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		},
			table.Entry("Test AddressPool object with default auto assign", "addresspool1", &metallbv1alpha1.AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "addresspool1",
					Namespace: OperatorNameSpace,
				},
				Spec: metallbv1alpha1.AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"1.1.1.1",
						"1.1.1.100",
					},
				},
			}, `address-pools:
- name: addresspool1
  protocol: layer2
  addresses:

  - 1.1.1.1
  - 1.1.1.100

`),
			table.Entry("Test AddressPool object with auto assign set to false", "addresspool2", &metallbv1alpha1.AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "addresspool2",
					Namespace: OperatorNameSpace,
				},
				Spec: metallbv1alpha1.AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"2.2.2.1",
						"2.2.2.100",
					},
					AutoAssign: &autoAssign,
				},
			}, `address-pools:
- name: addresspool2
  protocol: layer2
  auto-assign: false
  addresses:

  - 2.2.2.1
  - 2.2.2.100

`))
	})
	Context("MetalLB contains incorrect data", func() {
		Context("MetalLB has incorrect name", func() {

			var metallb *metallbv1beta1.MetalLB
			BeforeEach(func() {
				var err error
				metallb, err = getMetalLB()
				Expect(err).ToNot(HaveOccurred())
				metallb.SetName("incorrectname")
				Expect(testclient.Client.Create(context.Background(), metallb)).Should(Succeed())
			})

			AfterEach(func() {
				deleteMetalLB(metallb)
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
				correct_metallb, err = getMetalLB()
				Expect(err).ToNot(HaveOccurred())
				Expect(testclient.Client.Create(context.Background(), correct_metallb)).Should(Succeed())

				incorrect_metallb, err = getMetalLB()
				Expect(err).ToNot(HaveOccurred())
				incorrect_metallb.SetName("incorrectname")
				Expect(testclient.Client.Create(context.Background(), incorrect_metallb)).Should(Succeed())
			})

			AfterEach(func() {
				deleteMetalLB(incorrect_metallb)
				deleteMetalLB(correct_metallb)
			})
			It("should have correct statuses", func() {
				By("checking MetalLB resource status", func() {
					Eventually(func() bool {
						instance := &metallbv1beta1.MetalLB{}
						err := testclient.Client.Get(context.TODO(), goclient.ObjectKey{Namespace: incorrect_metallb.Namespace, Name: incorrect_metallb.Name}, instance)
						Expect(err).ToNot(HaveOccurred())
						return checkConditionStatus(instance) == status.ConditionDegraded
					}, 30*time.Second, 5*time.Second).Should(BeTrue())

					Eventually(func() bool {
						instance := &metallbv1beta1.MetalLB{}
						err := testclient.Client.Get(context.TODO(), goclient.ObjectKey{Namespace: correct_metallb.Namespace, Name: correct_metallb.Name}, instance)
						Expect(err).ToNot(HaveOccurred())
						return checkConditionStatus(instance) == status.ConditionAvailable
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
						return checkConditionStatus(instance) == status.ConditionAvailable
					}, 30*time.Second, 5*time.Second).Should(BeTrue())
				})
			})
		})
	})
	Context("Testing create/delete Multiple AddressPools", func() {
		It("should have created, merged and deleted resources correctly", func() {
			By("Creating first addresspool object ", func() {
				addresspool := &metallbv1alpha1.AddressPool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "addresspool1",
						Namespace: OperatorNameSpace,
					},
					Spec: metallbv1alpha1.AddressPoolSpec{
						Protocol: "layer2",
						Addresses: []string{
							"1.1.1.1",
							"1.1.1.100",
						},
					},
				}

				Expect(testclient.Client.Create(context.Background(), addresspool)).Should(Succeed())

				key := types.NamespacedName{
					Name:      "addresspool1",
					Namespace: OperatorNameSpace,
				}
				// Create addresspool resource
				By("By checking AddressPool1 resource is created")
				Eventually(func() error {
					err := testclient.Client.Get(context.Background(), key, addresspool)
					return err
				}, timeout, interval).Should(Succeed())

				// Checking ConfigMap is created
				By("By checking ConfigMap is created and matches addresspool1 configuration")
				Eventually(func() (string, error) {
					configmap, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
					if err != nil {
						return "", err
					}
					return configmap.Data[consts.MetalLBConfigMapName], err
				}, timeout, interval).Should(MatchYAML(`address-pools:
- name: addresspool1
  protocol: layer2
  addresses:

  - 1.1.1.1
  - 1.1.1.100

`))

			})

			By("Creating second addresspool object ", func() {
				addresspool := &metallbv1alpha1.AddressPool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "addresspool2",
						Namespace: OperatorNameSpace,
					},
					Spec: metallbv1alpha1.AddressPoolSpec{
						Protocol: "layer2",
						Addresses: []string{
							"2.2.2.1",
							"2.2.2.100",
						},
						AutoAssign: &autoAssign,
					},
				}

				Expect(testclient.Client.Create(context.Background(), addresspool)).Should(Succeed())

				key := types.NamespacedName{
					Name:      "addresspool2",
					Namespace: OperatorNameSpace,
				}
				// Create addresspool resource
				By("By checking AddressPool2 resource is created")
				Eventually(func() error {
					err := testclient.Client.Get(context.Background(), key, addresspool)
					return err
				}, timeout, interval).Should(Succeed())

				// Checking ConfigMap is created
				By("By checking ConfigMap is created and matches addresspool2 configuration")
				Eventually(func() (string, error) {
					configmap, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
					if err != nil {
						return "", err
					}
					return configmap.Data[consts.MetalLBConfigMapName], err
				}, timeout, interval).Should(MatchYAML(`address-pools:
- name: addresspool1
  protocol: layer2
  addresses:

  - 1.1.1.1
  - 1.1.1.100

- name: addresspool2
  protocol: layer2
  auto-assign: false
  addresses:

  - 2.2.2.1
  - 2.2.2.100

`))
			})

			By("Deleting the first addresspool object", func() {
				addresspool := &metallbv1alpha1.AddressPool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "addresspool1",
						Namespace: OperatorNameSpace,
					},
					Spec: metallbv1alpha1.AddressPoolSpec{
						Protocol: "layer2",
						Addresses: []string{
							"1.1.1.1",
							"1.1.1.100",
						},
					},
				}
				Eventually(func() bool {
					err := testclient.Client.Delete(context.Background(), addresspool)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue(), "Failed to delete AddressPool custom resource")

				By("By checking ConfigMap matches the expected configuration")
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
				}, timeout, interval).Should(MatchYAML(`address-pools:
- name: addresspool2
  protocol: layer2
  auto-assign: false
  addresses:

  - 2.2.2.1
  - 2.2.2.100

`))

			})

			By("Deleting the second addresspool object", func() {
				addresspool := &metallbv1alpha1.AddressPool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "addresspool2",
						Namespace: OperatorNameSpace,
					},
					Spec: metallbv1alpha1.AddressPoolSpec{
						Protocol: "layer2",
						Addresses: []string{
							"2.2.2.1",
							"2.2.2.100",
						},
					},
				}
				Eventually(func() bool {
					err := testclient.Client.Delete(context.Background(), addresspool)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue(), "Failed to delete AddressPool custom resource")

				By("By checking ConfigMap matches the expected configuration")
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
				}, timeout, interval).Should(MatchYAML(`
`))

			})

			// Make sure Configmap is deleted at the end of this test
			By("By checking ConfigMap is deleted at the end of the test")
			Eventually(func() bool {
				_, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Testing Update AddressPool", func() {
		It("should have created, update and finally delete addresspool correctly", func() {
			By("Creating addresspool object ", func() {
				addresspool := &metallbv1alpha1.AddressPool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "addresspool1",
						Namespace: OperatorNameSpace,
					},
					Spec: metallbv1alpha1.AddressPoolSpec{
						Protocol: "layer2",
						Addresses: []string{
							"1.1.1.1",
							"1.1.1.100",
						},
					},
				}

				Expect(testclient.Client.Create(context.Background(), addresspool)).Should(Succeed())

				key := types.NamespacedName{
					Name:      "addresspool1",
					Namespace: OperatorNameSpace,
				}
				// Create addresspool resource
				By("By checking AddressPool resource is created")
				Eventually(func() error {
					err := testclient.Client.Get(context.Background(), key, addresspool)
					return err
				}, timeout, interval).Should(Succeed())

				// Checking ConfigMap is created
				By("By checking ConfigMap is created and matches addresspool configuration")
				Eventually(func() (string, error) {
					configmap, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
					if err != nil {
						return "", err
					}
					return configmap.Data[consts.MetalLBConfigMapName], err
				}, timeout, interval).Should(MatchYAML(`address-pools:
- name: addresspool1
  protocol: layer2
  addresses:

  - 1.1.1.1
  - 1.1.1.100

`))

			})

			By("Update the same addresspool object with different range ", func() {
				addresspool := &metallbv1alpha1.AddressPool{}
				key := types.NamespacedName{
					Name:      "addresspool1",
					Namespace: OperatorNameSpace,
				}
				Eventually(func() error {
					err := testclient.Client.Get(context.Background(), key, addresspool)
					return err
				}, timeout, interval).Should(Succeed())

				addresspool.Spec = metallbv1alpha1.AddressPoolSpec{
					Protocol: "layer2",
					Addresses: []string{
						"1.1.1.1",
						"1.1.1.200",
					},
					AutoAssign: &autoAssign,
				}

				Eventually(func() error {
					err := testclient.Client.Update(context.Background(), addresspool)
					return err
				}, timeout, interval).Should(Succeed())

				// Checking ConfigMap is updated
				By("By checking ConfigMap is created and matches updated configuration")
				Eventually(func() (string, error) {
					configmap, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
					if err != nil {
						return "", err
					}
					return configmap.Data[consts.MetalLBConfigMapName], err
				}, timeout, interval).Should(MatchYAML(`address-pools:
- name: addresspool1
  protocol: layer2
  auto-assign: false
  addresses:

  - 1.1.1.1
  - 1.1.1.200

`))
			})

			By("Deleting the addresspool object", func() {
				addresspool := &metallbv1alpha1.AddressPool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "addresspool1",
						Namespace: OperatorNameSpace,
					},
					Spec: metallbv1alpha1.AddressPoolSpec{
						Protocol: "layer2",
						Addresses: []string{
							"1.1.1.1",
							"1.1.1.200",
						},
					},
				}
				Eventually(func() bool {
					err := testclient.Client.Delete(context.Background(), addresspool)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue(), "Failed to delete AddressPool custom resource")

				By("Checking ConfigMap is deleted")
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
				}, timeout, interval).Should(MatchYAML(`
`))

			})

			// Make sure Configmap is deleted at the end of this test
			By("Checking ConfigMap is deleted at the end of the test")
			Eventually(func() bool {
				_, err := testclient.Client.ConfigMaps(OperatorNameSpace).Get(context.Background(), consts.MetalLBConfigMapName, metav1.GetOptions{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})
})

func checkConditionStatus(instance *metallbv1beta1.MetalLB) string {
	availableStatus := false
	degradedStatus := false
	for _, condition := range instance.Status.Conditions {
		if condition.Type == status.ConditionDegraded && condition.Status == metav1.ConditionTrue {
			degradedStatus = true
		}
		if condition.Type == status.ConditionAvailable && condition.Status == metav1.ConditionTrue {
			availableStatus = true
		}
	}
	if availableStatus && !degradedStatus {
		return status.ConditionAvailable
	}
	if !availableStatus && degradedStatus {
		return status.ConditionDegraded
	}
	return ""
}

func decodeYAML(r io.Reader, obj interface{}) error {
	decoder := yaml.NewYAMLToJSONDecoder(r)
	return decoder.Decode(obj)
}

func loadMetalLBFromFile(metallb *metallbv1beta1.MetalLB, fileName string) error {
	f, err := os.Open(fmt.Sprintf("../../config/samples/%s", fileName))
	if err != nil {
		return err
	}
	defer f.Close()

	return decodeYAML(f, metallb)
}

// Delete and check the MetalLB custom resource is deleted to avoid status leak in between tests.
func deleteMetalLB(metallb *metallbv1beta1.MetalLB) {
	err := testclient.Client.Delete(context.Background(), metallb)
	if errors.IsNotFound(err) { // Ignore err, could be already deleted.
		return
	}
	Expect(err).ToNot(HaveOccurred())

	Eventually(func() bool {
		err := testclient.Client.Get(context.Background(), goclient.ObjectKey{Namespace: metallb.Namespace, Name: metallb.Name}, metallb)
		return errors.IsNotFound(err)
	}, 1*time.Minute, 5*time.Second).Should(BeTrue(), "Failed to delete MetalLB custom resource")

	Eventually(func() bool {
		_, err := testclient.Client.Deployments(metallb.Namespace).Get(context.Background(), consts.MetalLBDeploymentName, metav1.GetOptions{})
		return errors.IsNotFound(err)
	}, timeout, interval).Should(BeTrue())

	Eventually(func() bool {
		_, err := testclient.Client.DaemonSets(metallb.Namespace).Get(context.Background(), consts.MetalLBDaemonsetName, metav1.GetOptions{})
		return errors.IsNotFound(err)
	}, timeout, interval).Should(BeTrue())

	Eventually(func() bool {
		pods, _ := testclient.Client.Pods(metallb.Namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("component=%s", consts.MetalLBDeploymentName)})
		return len(pods.Items) == 0
	}, timeout, interval).Should(BeTrue())

	Eventually(func() bool {
		pods, _ := testclient.Client.Pods(metallb.Namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("component=%s", consts.MetalLBDaemonsetName)})
		return len(pods.Items) == 0
	}, timeout, interval).Should(BeTrue())
}

func getMetalLB() (*metallbv1beta1.MetalLB, error) {
	if !UseMetallbResourcesFromFile {
		return &metallbv1beta1.MetalLB{ObjectMeta: metav1.ObjectMeta{Name: "metallb", Namespace: OperatorNameSpace}}, nil
	}
	metallb := &metallbv1beta1.MetalLB{}
	if err := loadMetalLBFromFile(metallb, consts.MetalLBCRFile); err != nil {
		return nil, err
	}
	metallb.SetNamespace(OperatorNameSpace)
	return metallb, nil
}
