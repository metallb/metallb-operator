package controllers

import (
	"context"
	"fmt"
	"time"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/test/consts"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("MetalLB Controller", func() {
	Context("syncMetalLB", func() {
		AfterEach(func() {
			err := cleanTestNamespace()
			Expect(err).ToNot(HaveOccurred())
		})

		BeforeEach(func() {
			reconciler.EnvConfig = defaultEnvConfig
		})

		DescribeTable("Should create manifests with images and namespace overriden", func(bgpType metallbv1beta1.BGPType) {

			metallb := &metallbv1beta1.MetalLB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "metallb",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: metallbv1beta1.MetalLBSpec{
					BGPBackend: bgpType,
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
				"cp-liveness":  speakerImage,
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

			metallb = &metallbv1beta1.MetalLB{}
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err = k8sClient.Get(context.Background(), types.NamespacedName{Name: "metallb", Namespace: MetalLBTestNameSpace}, metallb)
				Expect(err).NotTo(HaveOccurred())
				By("Specify the SpeakerNodeSelector")
				metallb.Spec.SpeakerNodeSelector = map[string]string{"kubernetes.io/os": "linux", "node-role.kubernetes.io/worker": "true"}
				return k8sClient.Update(context.TODO(), metallb)
			})
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
			metallb = &metallbv1beta1.MetalLB{}
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err = k8sClient.Get(context.Background(), types.NamespacedName{Name: "metallb", Namespace: MetalLBTestNameSpace}, metallb)
				Expect(err).NotTo(HaveOccurred())
				metallb.Spec.SpeakerNodeSelector = map[string]string{}
				return k8sClient.Update(context.TODO(), metallb)
			})
			Expect(err).NotTo(HaveOccurred())

			metallb = &metallbv1beta1.MetalLB{}
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err = k8sClient.Get(context.Background(), types.NamespacedName{Name: "metallb", Namespace: MetalLBTestNameSpace}, metallb)
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
				return k8sClient.Update(context.TODO(), metallb)
			})
			Expect(err).NotTo(HaveOccurred())

			metallb = &metallbv1beta1.MetalLB{}
			err = k8sClient.Get(context.Background(), types.NamespacedName{Name: "metallb", Namespace: MetalLBTestNameSpace}, metallb)
			Expect(err).NotTo(HaveOccurred())
			speakerDaemonSet = &appsv1.DaemonSet{}
			expectedTolerations := []v1.Toleration{
				{
					Key:               "node-role.kubernetes.io/master",
					Operator:          "Exists",
					Value:             "",
					Effect:            "NoSchedule",
					TolerationSeconds: nil,
				},
				{
					Key:               "node-role.kubernetes.io/control-plane",
					Operator:          "Exists",
					Value:             "",
					Effect:            "NoSchedule",
					TolerationSeconds: nil,
				},
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
			Eventually(func() []v1.Toleration {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: consts.MetalLBDaemonsetName, Namespace: MetalLBTestNameSpace}, speakerDaemonSet)
				if err != nil {
					return nil
				}
				return speakerDaemonSet.Spec.Template.Spec.Tolerations
			}, 2*time.Second, 200*time.Millisecond).Should(Equal(expectedTolerations))
			Expect(speakerDaemonSet).NotTo(BeZero())
			Expect(len(speakerDaemonSet.Spec.Template.Spec.Containers)).To(BeNumerically(">", 0))
			// Reset toleration configuration
			metallb = &metallbv1beta1.MetalLB{}
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err = k8sClient.Get(context.Background(), types.NamespacedName{Name: "metallb", Namespace: MetalLBTestNameSpace}, metallb)
				Expect(err).NotTo(HaveOccurred())
				metallb.Spec.SpeakerTolerations = nil
				return k8sClient.Update(context.TODO(), metallb)
			})
			Expect(err).NotTo(HaveOccurred())
		},
			Entry("Native Mode", metallbv1beta1.NativeMode),
			Entry("FRR Mode", metallbv1beta1.FRRMode),
			Entry("FRR-K8s Mode", metallbv1beta1.FRRK8sMode),
		)

		DescribeTable("Should forward logLevel to containers", func(bgpType metallbv1beta1.BGPType) {

			metallb := &metallbv1beta1.MetalLB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "metallb",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: metallbv1beta1.MetalLBSpec{
					LogLevel:   metallbv1beta1.LogLevelWarn,
					BGPBackend: bgpType,
				},
			}

			err := k8sClient.Create(context.Background(), metallb)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() []v1.Container {
				speakerDaemonSet := &appsv1.DaemonSet{}
				err := k8sClient.Get(
					context.Background(),
					types.NamespacedName{Name: consts.MetalLBDaemonsetName, Namespace: MetalLBTestNameSpace},
					speakerDaemonSet)
				if err != nil {
					return nil
				}

				return speakerDaemonSet.Spec.Template.Spec.Containers
			}, 2*time.Second, 200*time.Millisecond).Should(
				ContainElement(
					And(
						WithTransform(nameGetter, Equal("speaker")),
						WithTransform(argsGetter, ContainElement("--log-level=warn")),
					)))

			controllerDeployment := &appsv1.Deployment{}
			Eventually(func() []v1.Container {
				err := k8sClient.Get(
					context.Background(),
					types.NamespacedName{Name: consts.MetalLBDeploymentName, Namespace: MetalLBTestNameSpace},
					controllerDeployment,
				)
				if err != nil {
					return nil
				}
				return controllerDeployment.Spec.Template.Spec.Containers
			}, 2*time.Second, 200*time.Millisecond).Should(
				ContainElement(
					And(
						WithTransform(nameGetter, Equal("controller")),
						WithTransform(argsGetter, ContainElement("--log-level=warn")),
					)))
		},
			Entry("Native Mode", metallbv1beta1.NativeMode),
			Entry("FRR Mode", metallbv1beta1.FRRMode),
			Entry("FRR-K8s Mode", metallbv1beta1.FRRK8sMode),
		)

		It("Should create manifests for frr-k8s", func() {
			metallb := &metallbv1beta1.MetalLB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "metallb",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: metallbv1beta1.MetalLBSpec{
					BGPBackend: metallbv1beta1.FRRK8sMode,
				},
			}

			frrk8sImage := "frr-k8s:test"
			frrImage := "test-frr:latest"
			kubeRbacImage := "test-kube-rbac-proxy:latest"

			frrk8sContainers := map[string]string{
				"controller":          frrk8sImage,
				"frr":                 frrImage,
				"reloader":            frrImage,
				"frr-metrics":         frrImage,
				"frr-status":          frrImage,
				"kube-rbac-proxy":     kubeRbacImage,
				"kube-rbac-proxy-frr": kubeRbacImage,
			}

			frrk8sInitContainers := map[string]string{
				"cp-frr-files":  frrImage,
				"cp-frr-status": frrk8sImage,
				"cp-reloader":   frrk8sImage,
				"cp-metrics":    frrk8sImage,
				"cp-liveness":   frrk8sImage,
			}

			By("Creating a MetalLB resource")
			err := k8sClient.Create(context.Background(), metallb)
			Expect(err).ToNot(HaveOccurred())

			By("Validating that the variables were templated correctly")
			frrk8sDaemonSet := &appsv1.DaemonSet{}
			Eventually(func() error {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: consts.FRRK8SDaemonsetName, Namespace: MetalLBTestNameSpace}, frrk8sDaemonSet)
				return err
			}, 2*time.Second, 200*time.Millisecond).ShouldNot((HaveOccurred()))
			Expect(frrk8sDaemonSet).NotTo(BeZero())
			Expect(len(frrk8sDaemonSet.Spec.Template.Spec.Containers)).To(BeNumerically(">", 0))
			for _, c := range frrk8sDaemonSet.Spec.Template.Spec.Containers {
				image, ok := frrk8sContainers[c.Name]
				Expect(ok).To(BeTrue(), fmt.Sprintf("container %s not found in %s", c.Name, frrk8sContainers))
				Expect(c.Image).To(Equal(image))
			}
			for _, c := range frrk8sDaemonSet.Spec.Template.Spec.InitContainers {
				image, ok := frrk8sInitContainers[c.Name]
				Expect(ok).To(BeTrue(), fmt.Sprintf("init container %s not found in %s", c.Name, frrk8sInitContainers))
				Expect(c.Image).To(Equal(image))
			}

		})
		It("Should switch between modes", func() {
			checkSpeakerBGPMode := func(mode metallbv1beta1.BGPType) {
				bgpTypeMatcher := ContainElement(v1.EnvVar{Name: "METALLB_BGP_TYPE", Value: string(mode)})
				// Since when running in native mode the helm chart doesn't set the type, here we
				// check for the absence of the env variable instead of having it set with a given value.
				if mode == metallbv1beta1.NativeMode {
					bgpTypeMatcher = Not(ContainElement(HaveField("Name", "METALLB_BGP_TYPE")))
				}

				EventuallyWithOffset(1, func() []v1.Container {
					speakerDaemonSet := &appsv1.DaemonSet{}
					err := k8sClient.Get(
						context.Background(),
						types.NamespacedName{Name: consts.MetalLBDaemonsetName, Namespace: MetalLBTestNameSpace},
						speakerDaemonSet)
					if err != nil {
						return nil
					}

					return speakerDaemonSet.Spec.Template.Spec.Containers
				}, 2*time.Second, 200*time.Millisecond).Should(
					ContainElement(
						And(
							WithTransform(nameGetter, Equal("speaker")),
							WithTransform(envGetter, bgpTypeMatcher),
						)))
			}

			metallb := &metallbv1beta1.MetalLB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "metallb",
					Namespace: MetalLBTestNameSpace,
				},
				Spec: metallbv1beta1.MetalLBSpec{
					BGPBackend: metallbv1beta1.FRRK8sMode,
				},
			}

			By("Creating a MetalLB resource with frr-k8s mode")
			err := k8sClient.Create(context.Background(), metallb)
			Expect(err).ToNot(HaveOccurred())

			By("Checking frr k8s is deployed")
			frrk8sDaemonSet := &appsv1.DaemonSet{}
			Eventually(func() error {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: consts.FRRK8SDaemonsetName, Namespace: MetalLBTestNameSpace}, frrk8sDaemonSet)
				return err
			}, 2*time.Second, 200*time.Millisecond).ShouldNot((HaveOccurred()))

			By("Checking the speaker is running in frr k8s mode")
			checkSpeakerBGPMode(metallbv1beta1.FRRK8sMode)

			By("Updating to frr mode")
			toUpdate := &metallbv1beta1.MetalLB{}
			err = k8sClient.Get(context.Background(), client.ObjectKey{Name: "metallb", Namespace: MetalLBTestNameSpace}, toUpdate)
			Expect(err).ToNot(HaveOccurred())
			toUpdate.Spec.BGPBackend = metallbv1beta1.FRRMode
			err = k8sClient.Update(context.Background(), toUpdate)
			Expect(err).ToNot(HaveOccurred())

			By("Checking frr k8s is not there")
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: consts.FRRK8SDaemonsetName, Namespace: MetalLBTestNameSpace}, frrk8sDaemonSet)
				return apierrors.IsNotFound(err)
			}, 5*time.Second, 200*time.Millisecond).Should(BeTrue())

			By("Checking the speaker is running in frr mode")
			checkSpeakerBGPMode(metallbv1beta1.FRRMode)

			By("Updating to native mode")
			toUpdate = &metallbv1beta1.MetalLB{}
			err = k8sClient.Get(context.Background(), client.ObjectKey{Name: "metallb", Namespace: MetalLBTestNameSpace}, toUpdate)
			Expect(err).ToNot(HaveOccurred())
			toUpdate.Spec.BGPBackend = metallbv1beta1.NativeMode
			err = k8sClient.Update(context.Background(), toUpdate)
			Expect(err).ToNot(HaveOccurred())

			By("Checking the speaker is running in native mode")
			checkSpeakerBGPMode(metallbv1beta1.NativeMode)

			By("Leaving the bgp backend empty")
			toUpdate = &metallbv1beta1.MetalLB{}
			err = k8sClient.Get(context.Background(), client.ObjectKey{Name: "metallb", Namespace: MetalLBTestNameSpace}, toUpdate)
			Expect(err).ToNot(HaveOccurred())
			toUpdate.Spec.BGPBackend = ""
			err = k8sClient.Update(context.Background(), toUpdate)
			Expect(err).ToNot(HaveOccurred())

			By("Checking the speaker is running in frr mode")
			checkSpeakerBGPMode(metallbv1beta1.FRRMode)
		})
	})
})

func cleanTestNamespace() error {
	err := k8sClient.DeleteAllOf(context.Background(), &metallbv1beta1.MetalLB{}, client.InNamespace(MetalLBTestNameSpace))
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
func argsGetter(c v1.Container) []string   { return c.Args }
func envGetter(c v1.Container) []v1.EnvVar { return c.Env }
func nameGetter(c v1.Container) string     { return c.Name }
