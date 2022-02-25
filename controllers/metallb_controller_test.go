package controllers

import (
	"context"
	"fmt"
	"time"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/test/consts"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("MetalLB Controller", func() {
	Context("syncMetalLB", func() {

		AfterEach(func() {
			err := cleanTestNamespace()
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should create manifests with images and namespace overriden", func() {

			metallb := &metallbv1beta1.MetalLB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "metallb",
					Namespace: MetalLBTestNameSpace,
				},
			}

			speakerImage := "test-speaker:latest"
			controllerImage := "test-controller:latest"
			frrImage := "test-frr:latest"
			kubeRbacImage := "test-kube-rbac-proxy:latest"

			controllerContainers := map[string]string{
				"controller":      controllerImage,
				"kube-rbac-proxy": kubeRbacImage,
			}

			speakerContainers := map[string]string{
				"speaker":             speakerImage,
				"frr":                 frrImage,
				"reloader":            frrImage,
				"frr-metrics":         frrImage,
				"kube-rbac-proxy":     kubeRbacImage,
				"kube-rbac-proxy-frr": kubeRbacImage,
			}

			speakerInitContainers := map[string]string{
				"cp-frr-files": frrImage,
				"cp-reloader":  speakerImage,
				"cp-metrics":   speakerImage,
			}

			By("Creating a MetalLB resource")
			err := k8sClient.Create(context.Background(), metallb)
			Expect(err).ToNot(HaveOccurred())

			By("Validating that the variables were templated correctly")
			controllerDeployment := &appsv1.Deployment{}
			Eventually(func() error {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: consts.MetalLBDeploymentName, Namespace: MetalLBTestNameSpace}, controllerDeployment)
				return err
			}, 2*time.Second, 200*time.Millisecond).ShouldNot((HaveOccurred()))
			Expect(controllerDeployment).NotTo(BeZero())
			Expect(len(controllerDeployment.Spec.Template.Spec.Containers)).To(BeNumerically(">", 0))
			for _, c := range controllerDeployment.Spec.Template.Spec.Containers {
				image, ok := controllerContainers[c.Name]
				Expect(ok).To(BeTrue(), fmt.Sprintf("container %s not found in %s", c.Name, controllerContainers))
				Expect(c.Image).To(Equal(image))
			}

			speakerDaemonSet := &appsv1.DaemonSet{}
			Eventually(func() error {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: consts.MetalLBDaemonsetName, Namespace: MetalLBTestNameSpace}, speakerDaemonSet)
				return err
			}, 2*time.Second, 200*time.Millisecond).ShouldNot((HaveOccurred()))
			Expect(speakerDaemonSet).NotTo(BeZero())
			Expect(len(speakerDaemonSet.Spec.Template.Spec.Containers)).To(BeNumerically(">", 0))
			for _, c := range speakerDaemonSet.Spec.Template.Spec.Containers {
				image, ok := speakerContainers[c.Name]
				Expect(ok).To(BeTrue(), fmt.Sprintf("container %s not found in %s", c.Name, speakerContainers))
				Expect(c.Image).To(Equal(image))
			}
			for _, c := range speakerDaemonSet.Spec.Template.Spec.InitContainers {
				image, ok := speakerInitContainers[c.Name]
				Expect(ok).To(BeTrue(), fmt.Sprintf("init container %s not found in %s", c.Name, speakerInitContainers))
				Expect(c.Image).To(Equal(image))
			}

			By("Specify the SpeakerNodeSelector")
			metallb.Spec.SpeakerNodeSelector = map[string]string{"node-role.kubernetes.io/worker": "true"}
			err = k8sClient.Update(context.TODO(), metallb)
			Expect(err).NotTo(HaveOccurred())
			speakerDaemonSet = &appsv1.DaemonSet{}
			Eventually(func() map[string]string {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: consts.MetalLBDaemonsetName, Namespace: MetalLBTestNameSpace}, speakerDaemonSet)
				if err != nil {
					return nil
				}
				return speakerDaemonSet.Spec.Template.Spec.NodeSelector
			}, 2*time.Second, 200*time.Millisecond).Should(Equal(metallb.Spec.SpeakerNodeSelector))
			Expect(speakerDaemonSet).NotTo(BeZero())
			Expect(len(speakerDaemonSet.Spec.Template.Spec.Containers)).To(BeNumerically(">", 0))
			// Reset nodeSelector configuration
			metallb.Spec.SpeakerNodeSelector = map[string]string{}
			err = k8sClient.Update(context.TODO(), metallb)
			Expect(err).NotTo(HaveOccurred())

			By("Specify the speaker's Tolerations")
			metallb.Spec.SpeakerTolerations = []v1.Toleration{
				{
					Key:      "example1",
					Operator: v1.TolerationOpExists,
					Effect:   v1.TaintEffectNoExecute,
				},
				{
					Key:      "example2",
					Operator: v1.TolerationOpExists,
					Effect:   v1.TaintEffectNoExecute,
				},
			}

			err = k8sClient.Update(context.TODO(), metallb)
			Expect(err).NotTo(HaveOccurred())

			speakerDaemonSet = &appsv1.DaemonSet{}
			Eventually(func() []v1.Toleration {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: consts.MetalLBDaemonsetName, Namespace: MetalLBTestNameSpace}, speakerDaemonSet)
				if err != nil {
					return nil
				}
				return speakerDaemonSet.Spec.Template.Spec.Tolerations
			}, 2*time.Second, 200*time.Millisecond).Should(Equal(metallb.Spec.SpeakerTolerations))
			Expect(speakerDaemonSet).NotTo(BeZero())
			Expect(len(speakerDaemonSet.Spec.Template.Spec.Containers)).To(BeNumerically(">", 0))
			// Reset toleration configuration
			metallb.Spec.SpeakerTolerations = nil
			err = k8sClient.Update(context.TODO(), metallb)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should forward logLevel to containers", func() {

			metallb := &metallbv1beta1.MetalLB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "metallb",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: metallbv1beta1.MetalLBSpec{
					LogLevel: metallbv1beta1.LogLevelWarn,
				},
			}

			err := k8sClient.Create(context.Background(), metallb)
			Expect(err).ToNot(HaveOccurred())

			speakerDaemonSet := &appsv1.DaemonSet{}
			Eventually(func() error {
				return k8sClient.Get(
					context.Background(),
					types.NamespacedName{Name: consts.MetalLBDaemonsetName, Namespace: MetalLBTestNameSpace},
					speakerDaemonSet)
			}, 2*time.Second, 200*time.Millisecond).ShouldNot((HaveOccurred()))

			Expect(speakerDaemonSet.Spec.Template.Spec.Containers).To(
				ContainElement(
					And(
						WithTransform(nameGetter, Equal("speaker")),
						WithTransform(argsGetter, ContainElement("--log-level=warn")),
					)))

			controllerDeployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(
					context.Background(),
					types.NamespacedName{Name: consts.MetalLBDeploymentName, Namespace: MetalLBTestNameSpace},
					controllerDeployment,
				)
			}, 2*time.Second, 200*time.Millisecond).ShouldNot((HaveOccurred()))

			Expect(controllerDeployment.Spec.Template.Spec.Containers).To(
				ContainElement(
					And(
						WithTransform(nameGetter, Equal("controller")),
						WithTransform(argsGetter, ContainElement("--log-level=warn")),
					)))
		})
	})
})

func cleanTestNamespace() error {
	err := k8sClient.DeleteAllOf(context.Background(), &metallbv1beta1.AddressPool{}, client.InNamespace(MetalLBTestNameSpace))
	if err != nil {
		return err
	}
	err = k8sClient.DeleteAllOf(context.Background(), &metallbv1beta1.BGPPeer{}, client.InNamespace(MetalLBTestNameSpace))
	if err != nil {
		return err
	}
	err = k8sClient.DeleteAllOf(context.Background(), &metallbv1beta1.BFDProfile{}, client.InNamespace(MetalLBTestNameSpace))
	if err != nil {
		return err
	}
	err = k8sClient.DeleteAllOf(context.Background(), &v1.ConfigMap{}, client.InNamespace(MetalLBTestNameSpace))
	if err != nil {
		return err
	}
	err = k8sClient.DeleteAllOf(context.Background(), &metallbv1beta1.MetalLB{}, client.InNamespace(MetalLBTestNameSpace))
	if err != nil {
		return err
	}
	err = k8sClient.DeleteAllOf(context.Background(), &appsv1.Deployment{}, client.InNamespace(MetalLBTestNameSpace))
	if err != nil {
		return err
	}
	err = k8sClient.DeleteAllOf(context.Background(), &appsv1.DaemonSet{}, client.InNamespace(MetalLBTestNameSpace))
	return err
}

// Gomega transformation functions for v1.Container
func argsGetter(c v1.Container) []string { return c.Args }
func nameGetter(c v1.Container) string   { return c.Name }
