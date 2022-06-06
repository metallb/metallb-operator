package tests

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/controllers"
	"github.com/metallb/metallb-operator/pkg/status"
	"github.com/metallb/metallb-operator/test/consts"
	testclient "github.com/metallb/metallb-operator/test/e2e/client"
	metallbutils "github.com/metallb/metallb-operator/test/e2e/metallb"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	goclient "sigs.k8s.io/controller-runtime/pkg/client"
)

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

		AfterEach(func() {
			// We recreate in case some test deleted it
			metallb, err := metallbutils.Get(OperatorNameSpace, UseMetallbResourcesFromFile)
			Expect(err).ToNot(HaveOccurred())
			err = testclient.Client.Create(context.Background(), metallb)
			if !errors.IsAlreadyExists(err) {
				Expect(err).ToNot(HaveOccurred())
			}

		})

		It("should have the correct owner references", func() {
			deployment, err := testclient.Client.Deployments(OperatorNameSpace).Get(context.Background(), consts.MetalLBDeploymentName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(deployment.OwnerReferences).ToNot(BeNil())
			Expect(deployment.OwnerReferences[0].Kind).To(Equal("MetalLB"))

			daemonset, err := testclient.Client.DaemonSets(OperatorNameSpace).Get(context.Background(), consts.MetalLBDaemonsetName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(daemonset.OwnerReferences).ToNot(BeNil())
			Expect(daemonset.OwnerReferences[0].Kind).To(Equal("MetalLB"))
		})

		It("should have MetalLB pods in running state", func() {
			By("checking MetalLB controller deployment is in running state", func() {
				Eventually(func() bool {
					deploy, err := testclient.Client.Deployments(OperatorNameSpace).Get(context.Background(), consts.MetalLBDeploymentName, metav1.GetOptions{})
					if err != nil {
						return false
					}
					return deploy.Status.ReadyReplicas > 0 && deploy.Status.ReadyReplicas == deploy.Status.Replicas
				}, metallbutils.DeployTimeout, metallbutils.Interval).Should(BeTrue())

				pods, err := testclient.Client.Pods(OperatorNameSpace).List(context.Background(), metav1.ListOptions{
					LabelSelector: "component=controller"})
				Expect(err).ToNot(HaveOccurred())

				deploy, err := testclient.Client.Deployments(OperatorNameSpace).Get(context.Background(), consts.MetalLBDeploymentName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(len(pods.Items)).To(Equal(int(deploy.Status.Replicas)))

				for _, pod := range pods.Items {
					Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
				}
			})

			By("checking MetalLB daemonset is in running state", func() {
				Eventually(func() bool {
					daemonset, err := testclient.Client.DaemonSets(OperatorNameSpace).Get(context.Background(), consts.MetalLBDaemonsetName, metav1.GetOptions{})
					if err != nil {
						return false
					}
					return daemonset.Status.DesiredNumberScheduled == daemonset.Status.NumberReady
				}, metallbutils.DeployTimeout, metallbutils.Interval).Should(BeTrue())

				pods, err := testclient.Client.Pods(OperatorNameSpace).List(context.Background(), metav1.ListOptions{
					LabelSelector: "component=speaker"})
				Expect(err).ToNot(HaveOccurred())

				daemonset, err := testclient.Client.DaemonSets(OperatorNameSpace).Get(context.Background(), consts.MetalLBDaemonsetName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(len(pods.Items)).To(Equal(int(daemonset.Status.DesiredNumberScheduled)))

				for _, pod := range pods.Items {
					Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
				}
			})
			By("checking MetalLB CR status is set", func() {
				Eventually(func() bool {
					config := &metallbv1beta1.MetalLB{}
					err := testclient.Client.Get(context.Background(), goclient.ObjectKey{Namespace: OperatorNameSpace, Name: controllers.DefaultMetalLBCrName}, config)
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

	Context("Deleting MetalLB", func() {
		It("should preserve the crds", func() {
			instance := &metallbv1beta1.MetalLB{}
			err := testclient.Client.Get(context.TODO(), goclient.ObjectKey{Namespace: OperatorNameSpace, Name: controllers.DefaultMetalLBCrName}, instance)
			Expect(err).ToNot(HaveOccurred())

			Consistently(func() error {
				crds := []string{
					"addresspools.metallb.io",
					"bfdprofiles.metallb.io",
					"bgpadvertisements.metallb.io",
					"bgppeers.metallb.io",
					"communities.metallb.io",
					"ipaddresspools.metallb.io",
					"l2advertisements.metallb.io",
				}
				for _, crd := range crds {
					_, err := testclient.Client.CustomResourceDefinitions().Get(context.Background(), crd, metav1.GetOptions{})
					if err != nil {
						return err
					}
				}
				return nil
			}, 30*time.Second, 5*time.Second).Should(Not(HaveOccurred()))

			Expect(err).ToNot(HaveOccurred())
		})
	})
})
