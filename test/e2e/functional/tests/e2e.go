package tests

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/pkg/status"
	"github.com/metallb/metallb-operator/test/consts"
	testclient "github.com/metallb/metallb-operator/test/e2e/client"
	"github.com/metallb/metallb-operator/test/e2e/metallb"
	metallbutils "github.com/metallb/metallb-operator/test/e2e/metallb"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	schv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	goclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	UseMetallbResourcesFromFile = false
	OperatorNameSpace           = consts.DefaultOperatorNameSpace
	isOpenshift                 = false
	frrk8sNamespace             = OperatorNameSpace
)

func init() {
	if len(os.Getenv("USE_LOCAL_RESOURCES")) != 0 {
		UseMetallbResourcesFromFile = true
	}

	if ns := os.Getenv("OO_INSTALL_NAMESPACE"); len(ns) != 0 {
		OperatorNameSpace = ns
	}
	if os.Getenv("IS_OPENSHIFT") != "" {
		isOpenshift = true
	}
	if ns := os.Getenv("FRRK8S_EXTERNAL_NAMESPACE"); ns != "" {
		frrk8sNamespace = ns
	}
}

var _ = Describe("metallb", func() {
	Context("MetalLB deploy should work when deployed", Ordered, func() {
		var metallb *metallbv1beta1.MetalLB
		var metallbCRExisted bool

		BeforeAll(func() {
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

		AfterAll(func() {
			if !metallbCRExisted {
				deployment, err := testclient.Client.Deployments(metallb.Namespace).Get(context.Background(), consts.MetalLBDeploymentName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(deployment.OwnerReferences).ToNot(BeNil())
				Expect(deployment.OwnerReferences[0].Kind).To(Equal("MetalLB"))

				daemonset, err := testclient.Client.DaemonSets(metallb.Namespace).Get(context.Background(), consts.MetalLBDaemonsetName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(daemonset.OwnerReferences).ToNot(BeNil())
				Expect(daemonset.OwnerReferences[0].Kind).To(Equal("MetalLB"))

				metallbutils.DeleteAndCheck(metallb)
			}
		})

		DescribeTable("with BGP type", func(bgpType metallbv1beta1.BGPType) {
			if isOpenshift && bgpType != metallbv1beta1.FRRK8sExternalMode {
				Skip("only frr-k8s-external is supported on OpenShift")
			}
			checkControllerBGPMode := func(mode metallbv1beta1.BGPType) {
				bgpTypeMatcher := ContainElement(corev1.EnvVar{Name: "METALLB_BGP_TYPE", Value: string(mode)})
				if mode == metallbv1beta1.NativeMode {
					bgpTypeMatcher = Not(ContainElement(HaveField("Name", "METALLB_BGP_TYPE")))
				}

				EventuallyWithOffset(1, func() []corev1.Container {
					controllerDeployment := &appsv1.Deployment{}
					err := testclient.Client.Get(
						context.Background(),
						types.NamespacedName{Name: consts.MetalLBDeploymentName, Namespace: metallb.Namespace},
						controllerDeployment)
					if err != nil {
						return nil
					}

					return controllerDeployment.Spec.Template.Spec.Containers
				}, 2*time.Second, 200*time.Millisecond).Should(
					ContainElement(
						And(
							WithTransform(nameGetter, Equal("controller")),
							WithTransform(envGetter, bgpTypeMatcher),
						)))
			}

			checkSpeakerBGPMode := func(mode metallbv1beta1.BGPType) {
				bgpTypeMatcher := ContainElement(corev1.EnvVar{Name: "METALLB_BGP_TYPE", Value: string(mode)})
				if mode == metallbv1beta1.NativeMode {
					bgpTypeMatcher = Not(ContainElement(HaveField("Name", "METALLB_BGP_TYPE")))
				}

				EventuallyWithOffset(1, func() []corev1.Container {
					speakerDaemonSet := &appsv1.DaemonSet{}
					err := testclient.Client.Get(
						context.Background(),
						types.NamespacedName{Name: consts.MetalLBDaemonsetName, Namespace: metallb.Namespace},
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

			By("setting the bgpType")

			Eventually(func() error {
				err := testclient.Client.Get(context.Background(), goclient.ObjectKey{Namespace: metallb.Namespace, Name: metallb.Name}, metallb)
				if !errors.IsNotFound(err) {
					Expect(err).ToNot(HaveOccurred())
				}

				if errors.IsNotFound(err) {
					metallb := &metallbv1beta1.MetalLB{
						ObjectMeta: metav1.ObjectMeta{
							Name:      metallb.Name,
							Namespace: metallb.Namespace,
						},
						Spec: metallbv1beta1.MetalLBSpec{
							LogLevel:   metallbv1beta1.LogLevelWarn,
							BGPBackend: bgpType,
						},
					}
					err := testclient.Client.Create(context.Background(), metallb)
					return err
				}
				metallb.Spec.BGPBackend = bgpType
				err = testclient.Client.Update(context.Background(), metallb)
				return err
			}, metallbutils.DeployTimeout, metallbutils.Interval).ShouldNot(HaveOccurred())

			By("checking the controller is running in the right bgp mode")
			expectedMode := bgpType
			if bgpType == metallbv1beta1.FRRK8sExternalMode {
				expectedMode = metallbv1beta1.FRRK8sMode
			}
			checkControllerBGPMode(expectedMode)

			By("checking MetalLB controller deployment is in running state")
			Eventually(func() error {
				deploy, err := testclient.Client.Deployments(metallb.Namespace).Get(context.Background(), consts.MetalLBDeploymentName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				pods, err := testclient.Client.Pods(OperatorNameSpace).List(context.Background(), metav1.ListOptions{
					LabelSelector: "component=controller"})
				if err != nil {
					return err
				}

				if len(pods.Items) != int(deploy.Status.Replicas) {
					return fmt.Errorf("deployment %s pods are not ready, expected %d replicas got %d pods", consts.MetalLBOperatorDeploymentName, deploy.Status.Replicas, len(pods.Items))
				}

				for _, pod := range pods.Items {
					if pod.Status.Phase != corev1.PodRunning {
						return fmt.Errorf("deployment %s pod %s is not running, expected status %s got %s", consts.MetalLBOperatorDeploymentName, pod.Name, corev1.PodRunning, pod.Status.Phase)
					}
				}

				return nil
			}, metallbutils.DeployTimeout, metallbutils.Interval).ShouldNot(HaveOccurred())

			By("checking the speaker is running in the right bgp mode")
			checkSpeakerBGPMode(expectedMode)

			By("checking MetalLB daemonset is in running state")
			Eventually(func() error {
				daemonset, err := testclient.Client.DaemonSets(metallb.Namespace).Get(context.Background(), consts.MetalLBDaemonsetName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				pods, err := testclient.Client.Pods(OperatorNameSpace).List(context.Background(), metav1.ListOptions{
					LabelSelector: "component=speaker"})
				if err != nil {
					return err
				}

				if len(pods.Items) != int(daemonset.Status.DesiredNumberScheduled) {
					return fmt.Errorf("daemonset %s pods are not ready, expected %d generations got %d pods", consts.MetalLBDaemonsetName, int(daemonset.Status.DesiredNumberScheduled), len(pods.Items))
				}

				for _, pod := range pods.Items {
					if pod.Status.Phase != corev1.PodRunning {
						return fmt.Errorf("daemonset %s pod %s is not running, expected status %s got %s", consts.MetalLBDaemonsetName, pod.Name, corev1.PodRunning, pod.Status.Phase)
					}
				}

				return nil
			}, metallbutils.DeployTimeout, metallbutils.Interval).ShouldNot(HaveOccurred())

			By("checking MetalLB CR status is set")
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

			if bgpType != metallbv1beta1.FRRK8sMode && bgpType != metallbv1beta1.FRRK8sExternalMode {
				return
			}
			By("checking frr-k8s daemonset is in running state")
			Eventually(func() error {
				daemonset, err := testclient.Client.DaemonSets(frrk8sNamespace).Get(context.Background(), consts.FRRK8SDaemonsetName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				pods, err := testclient.Client.Pods(frrk8sNamespace).List(context.Background(), metav1.ListOptions{
					LabelSelector: "component=frr-k8s"})
				if err != nil {
					return err
				}

				if len(pods.Items) != int(daemonset.Status.DesiredNumberScheduled) {
					return fmt.Errorf("daemonset %s pods are not ready, expected %d generations got %d pods", consts.FRRK8SDaemonsetName, int(daemonset.Status.DesiredNumberScheduled), len(pods.Items))
				}

				for _, pod := range pods.Items {
					if pod.Status.Phase != corev1.PodRunning {
						return fmt.Errorf("daemonset %s pod %s is not running, expected status %s got %s", consts.FRRK8SDaemonsetName, pod.Name, corev1.PodRunning, pod.Status.Phase)
					}
				}

				return nil
			}, metallbutils.DeployTimeout, metallbutils.Interval).ShouldNot(HaveOccurred())

			By("checking frr-k8s webhook deployment is in running state")
			Eventually(func() error {
				deploy, err := testclient.Client.Deployments(frrk8sNamespace).Get(context.Background(), consts.FRRK8SWebhookDeploymentName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				pods, err := testclient.Client.Pods(frrk8sNamespace).List(context.Background(), metav1.ListOptions{
					LabelSelector: "component=frr-k8s-webhook-server"})
				if err != nil {
					return err
				}

				if len(pods.Items) != int(deploy.Status.Replicas) {
					return fmt.Errorf("deployment %s pods are not ready, expected %d replicas got %d pods", consts.FRRK8SWebhookDeploymentName, deploy.Status.Replicas, len(pods.Items))
				}

				for _, pod := range pods.Items {
					if pod.Status.Phase != corev1.PodRunning {
						return fmt.Errorf("deployment %s pod %s is not running, expected status %s got %s", consts.FRRK8SWebhookDeploymentName, pod.Name, corev1.PodRunning, pod.Status.Phase)
					}
				}

				return nil
			}, metallbutils.DeployTimeout, metallbutils.Interval).ShouldNot(HaveOccurred())

		},
			Entry("Native Mode", metallbv1beta1.NativeMode),
			Entry("FRR Mode", metallbv1beta1.FRRMode),
			Entry("FRR-K8s Mode", metallbv1beta1.FRRK8sMode),
			Entry("FRR-K8s-external Mode", metallbv1beta1.FRRK8sExternalMode),
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
				By("checking MetalLB resource status")
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
				metallbutils.DeleteAndCheck(correct_metallb)
			})
			It("should have correct statuses", func() {
				By("checking MetalLB resource status")
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
				}, metallb.DeployTimeout, 5*time.Second).Should(BeTrue())

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

	Context("MetalLB configured extra config parameters", func() {
		var correct_metallb *metallbv1beta1.MetalLB
		var priorityClass *schv1.PriorityClass
		priorityClassName := "high-priority"
		BeforeEach(func() {
			var err error
			correct_metallb, err = metallbutils.Get(OperatorNameSpace, UseMetallbResourcesFromFile)
			Expect(err).ToNot(HaveOccurred())
			priorityClass = metallbutils.NewPriorityClass(priorityClassName, 10000)

			Expect(testclient.Client.Create(context.Background(), priorityClass)).Should(Succeed())
		})

		AfterEach(func() {
			metallbutils.DeleteAndCheck(correct_metallb)
			metallbutils.DeletePriorityClass(priorityClass)
		})

		It("set with additional parameters", func() {
			By("create and validate resources")
			metallb := metallbutils.New(OperatorNameSpace, func(m *metallbv1beta1.MetalLB) {
				m.Spec.SpeakerConfig = &metallbv1beta1.Config{
					PriorityClassName: priorityClass.GetName(),
					Annotations:       map[string]string{"test": "e2e"},
					Resources:         &v1.ResourceRequirements{Limits: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: *resource.NewMilliQuantity(100, resource.DecimalSI)}},
				}
				m.Spec.ControllerConfig = &metallbv1beta1.Config{
					PriorityClassName: priorityClass.GetName(),
					Annotations:       map[string]string{"test": "e2e"},
					Resources:         &v1.ResourceRequirements{Limits: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: *resource.NewMilliQuantity(100, resource.DecimalSI)}},
					Affinity: &v1.Affinity{PodAffinity: &v1.PodAffinity{RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{{LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"component": "controller",
						}},
						TopologyKey: "kubernetes.io/hostname"}}}},
				}
			})
			Expect(testclient.Client.Create(context.Background(), metallb)).Should(Succeed())

			Eventually(func() error {
				controller, err := testclient.Client.Deployments(metallb.Namespace).Get(context.Background(), consts.MetalLBDeploymentName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				pods, err := testclient.Client.Pods(OperatorNameSpace).List(context.Background(), metav1.ListOptions{
					LabelSelector: "component=controller"})
				if err != nil {
					return err
				}

				if len(pods.Items) != int(controller.Status.Replicas) {
					return fmt.Errorf("deployment %s pods are not ready, expected %d replicas got %d pods", consts.MetalLBOperatorDeploymentName, controller.Status.Replicas, len(pods.Items))
				}

				var controllerContainerFound bool
				for _, pod := range pods.Items {
					if pod.Status.Phase != corev1.PodRunning {
						return fmt.Errorf("deployment %s pod %s is not running, expected status %s got %s", consts.MetalLBOperatorDeploymentName, pod.Name, corev1.PodRunning, pod.Status.Phase)
					}

					for _, container := range pod.Spec.Containers {
						if container.Name == "controller" {
							if container.Resources.Limits.Cpu().MilliValue() != int64(100) {
								return fmt.Errorf("controller CPU limit should be 100")
							}
							controllerContainerFound = true
						}
					}
				}

				if !controllerContainerFound {
					return fmt.Errorf("controller container not found")
				}

				if controller.Spec.Template.Spec.PriorityClassName != priorityClassName {
					return fmt.Errorf("controller PriorityClassName different than '%s'", priorityClassName)
				}

				if controller.Spec.Template.Annotations["test"] != "e2e" {
					return fmt.Errorf("controller test annotation different than 'e2e'")
				}

				if controller.Spec.Template.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchLabels["component"] != "controller" {
					return fmt.Errorf("controller LabelSelector different than 'controller'")
				}

				if controller.Spec.Template.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey != "kubernetes.io/hostname" {
					return fmt.Errorf("controller TopologyKey different than 'kubernetes.io/hostname'")
				}

				return nil
			}, metallbutils.DeployTimeout, metallbutils.Interval).ShouldNot(HaveOccurred())

			Eventually(func() error {
				daemonset, err := testclient.Client.DaemonSets(metallb.Namespace).Get(context.Background(), consts.MetalLBDaemonsetName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				pods, err := testclient.Client.Pods(OperatorNameSpace).List(context.Background(), metav1.ListOptions{
					LabelSelector: "component=speaker"})
				if err != nil {
					return err
				}

				if len(pods.Items) != int(daemonset.Status.DesiredNumberScheduled) {
					return fmt.Errorf("daemonset %s pods are not ready, expected %d generations got %d pods", consts.MetalLBDaemonsetName, int(daemonset.Status.DesiredNumberScheduled), len(pods.Items))
				}

				var speakerContainerFound bool
				for _, pod := range pods.Items {
					if pod.Status.Phase != corev1.PodRunning {
						return fmt.Errorf("daemonset %s pod %s is not running, expected status %s got %s", consts.MetalLBDaemonsetName, pod.Name, corev1.PodRunning, pod.Status.Phase)
					}

					for _, container := range pod.Spec.Containers {
						if container.Name == "speaker" {
							if container.Resources.Limits.Cpu().MilliValue() != int64(100) {
								return fmt.Errorf("speaker CPU limit should be 100")
							}
							speakerContainerFound = true
						}
					}
				}

				if !speakerContainerFound {
					return fmt.Errorf("speaker container not found")
				}

				if daemonset.Spec.Template.Spec.PriorityClassName != priorityClassName {
					return fmt.Errorf("speaker PriorityClassName different than '%s'", priorityClassName)
				}

				if daemonset.Spec.Template.Annotations["test"] != "e2e" {
					return fmt.Errorf("speaker test annotation different than 'e2e'")
				}

				return nil
			}, metallbutils.DeployTimeout, metallbutils.Interval).ShouldNot(HaveOccurred())

			metallbutils.DeleteAndCheck(metallb)
		})
	})

	Context("Update MetalLB resources", func() {
		var metallb *metallbv1beta1.MetalLB
		var priorityClass *schv1.PriorityClass
		priorityClassName := "high-priority"
		BeforeEach(func() {
			var err error
			metallb, err = metallbutils.Get(OperatorNameSpace, UseMetallbResourcesFromFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(testclient.Client.Create(context.Background(), metallb)).Should(Succeed())
			priorityClass = metallbutils.NewPriorityClass(priorityClassName, 10000)
			Expect(testclient.Client.Create(context.Background(), priorityClass)).Should(Succeed())
		})

		AfterEach(func() {
			metallbutils.DeleteAndCheck(metallb)
			metallbutils.DeletePriorityClass(priorityClass)
		})
		It("patch additional parameters", func() {
			Eventually(func() error {
				controller, err := testclient.Client.Deployments(metallb.Namespace).Get(context.Background(), consts.MetalLBDeploymentName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				pods, err := testclient.Client.Pods(OperatorNameSpace).List(context.Background(), metav1.ListOptions{
					LabelSelector: "component=controller"})
				if err != nil {
					return err
				}

				if len(pods.Items) != int(controller.Status.Replicas) {
					return fmt.Errorf("deployment %s pods are not ready, expected %d replicas got %d pods", consts.MetalLBOperatorDeploymentName, controller.Status.Replicas, len(pods.Items))
				}

				var controllerContainerFound bool
				for _, pod := range pods.Items {
					if pod.Status.Phase != corev1.PodRunning {
						return fmt.Errorf("deployment %s pod %s is not running, expected status %s got %s", consts.MetalLBOperatorDeploymentName, pod.Name, corev1.PodRunning, pod.Status.Phase)
					}

					for _, container := range pod.Spec.Containers {
						if container.Name == "controller" {
							if container.Resources.Limits.Cpu().MilliValue() != int64(0) {
								return fmt.Errorf("controller CPU limit should be 0")
							}
							controllerContainerFound = true
						}
					}
				}

				if !controllerContainerFound {
					return fmt.Errorf("controller container not found")
				}

				if controller.Spec.Template.Spec.PriorityClassName != "" {
					return fmt.Errorf("controller PriorityClassName should not be set")
				}

				if controller.Spec.Template.Annotations["test"] != "" {
					return fmt.Errorf("controller test annotation should not be set")
				}

				return nil
			}, metallbutils.DeployTimeout, metallbutils.Interval).ShouldNot(HaveOccurred())

			Eventually(func() error {
				daemonset, err := testclient.Client.DaemonSets(metallb.Namespace).Get(context.Background(), consts.MetalLBDaemonsetName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				pods, err := testclient.Client.Pods(OperatorNameSpace).List(context.Background(), metav1.ListOptions{
					LabelSelector: "component=speaker"})
				if err != nil {
					return err
				}

				if len(pods.Items) != int(daemonset.Status.DesiredNumberScheduled) {
					return fmt.Errorf("daemonset %s pods are not ready, expected %d generations got %d pods", consts.MetalLBDaemonsetName, int(daemonset.Status.DesiredNumberScheduled), len(pods.Items))
				}

				for _, pod := range pods.Items {
					if pod.Status.Phase != corev1.PodRunning {
						return fmt.Errorf("daemonset %s pod %s is not running, expected status %s got %s", consts.MetalLBDaemonsetName, pod.Name, corev1.PodRunning, pod.Status.Phase)
					}
				}

				return nil
			}, metallbutils.DeployTimeout, metallbutils.Interval).ShouldNot(HaveOccurred())

			instance := &metallbv1beta1.MetalLB{}
			err := testclient.Client.Get(context.TODO(), goclient.ObjectKey{Namespace: metallb.Namespace, Name: metallb.Name}, instance)
			Expect(err).ToNot(HaveOccurred())
			instance.Spec.ControllerConfig = &metallbv1beta1.Config{
				PriorityClassName: priorityClass.GetName(),
				Annotations:       map[string]string{"test": "e2e"},
				Resources:         &v1.ResourceRequirements{Limits: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: *resource.NewMilliQuantity(100, resource.DecimalSI)}},
				Affinity: &v1.Affinity{PodAffinity: &v1.PodAffinity{RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{{LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"component": "controller",
					}},
					TopologyKey: "kubernetes.io/hostname"}}}},
			}
			err = testclient.Client.Update(context.TODO(), instance)
			Expect(err).ToNot(HaveOccurred())

			By("checking MetalLB resource status")
			Eventually(func() error {
				controller, err := testclient.Client.Deployments(metallb.Namespace).Get(context.Background(), consts.MetalLBDeploymentName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				pods, err := testclient.Client.Pods(OperatorNameSpace).List(context.Background(), metav1.ListOptions{
					LabelSelector: "component=controller"})
				if err != nil {
					return err
				}

				if len(pods.Items) != int(controller.Status.Replicas) {
					return fmt.Errorf("deployment %s pods are not ready, expected %d replicas got %d pods", consts.MetalLBOperatorDeploymentName, controller.Status.Replicas, len(pods.Items))
				}

				if controller.Spec.Template.Spec.PriorityClassName != priorityClassName {
					return fmt.Errorf("controller PriorityClassName different than '%s'", priorityClassName)
				}

				if controller.Spec.Template.Annotations["test"] != "e2e" {
					return fmt.Errorf("controller test annotation different than 'e2e'")
				}

				if controller.Spec.Template.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchLabels["component"] != "controller" {
					return fmt.Errorf("controller LabelSelector different than 'controller'")
				}

				if controller.Spec.Template.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey != "kubernetes.io/hostname" {
					return fmt.Errorf("controller TopologyKey different than 'kubernetes.io/hostname'")
				}

				for _, pod := range pods.Items {
					for _, container := range pod.Spec.Containers {
						if container.Name == "controller" {
							if container.Resources.Limits.Cpu().MilliValue() != int64(100) {
								return fmt.Errorf("controller CPU limit should be 100")
							}
						}
					}
				}

				return nil
			}, metallbutils.DeployTimeout, metallbutils.Interval).ShouldNot(HaveOccurred())
		})
	})

	Context("Invalid MetalLB resources", func() {
		var correct_metallb *metallbv1beta1.MetalLB
		BeforeEach(func() {
			var err error
			correct_metallb, err = metallbutils.Get(OperatorNameSpace, UseMetallbResourcesFromFile)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			metallbutils.DeleteAndCheck(correct_metallb)
		})
		It("validate create with incorrect toleration", func() {
			metallb := metallbutils.New(OperatorNameSpace, func(m *metallbv1beta1.MetalLB) {
				tolerations := []corev1.Toleration{{Effect: corev1.TaintEffectNoSchedule, Key: "group",
					Operator: corev1.TolerationOpEqual, TolerationSeconds: &resource.MaxMilliValue,
					Value: "infra"}}
				m.Spec.ControllerTolerations = tolerations
			})
			Expect(testclient.Client.Create(context.Background(), metallb)).ShouldNot(Succeed())
		})
		It("validate update with incorrect toleration", func() {
			Expect(testclient.Client.Create(context.Background(), correct_metallb)).Should(Succeed())
			instance := &metallbv1beta1.MetalLB{}
			By("checking MetalLB CR status is set")
			Eventually(func() bool {
				err := testclient.Client.Get(context.Background(), goclient.ObjectKey{Namespace: correct_metallb.Namespace, Name: correct_metallb.Name}, instance)
				Expect(err).ToNot(HaveOccurred())
				if instance.Status.Conditions == nil {
					return false
				}
				for _, condition := range instance.Status.Conditions {
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
			instance.Spec.SpeakerTolerations = []corev1.Toleration{{Effect: corev1.TaintEffectNoSchedule, Key: "group",
				Operator: corev1.TolerationOpEqual, TolerationSeconds: &resource.MaxMilliValue,
				Value: "infra"}}
			Expect(testclient.Client.Update(context.Background(), instance)).ShouldNot(Succeed())
		})
		It("validate incorrect affinity", func() {
			metallb := metallbutils.New(OperatorNameSpace, func(m *metallbv1beta1.MetalLB) {
				m.Spec.ControllerConfig = &metallbv1beta1.Config{
					Affinity: &v1.Affinity{NodeAffinity: &v1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
						{Weight: 0, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{{Key: "zone",
							Operator: v1.NodeSelectorOpIn, Values: []string{"east"}}}}},
					}}},
				}
			})
			Expect(testclient.Client.Create(context.Background(), metallb)).ShouldNot(Succeed())
			metallb = metallbutils.New(OperatorNameSpace, func(m *metallbv1beta1.MetalLB) {
				m.Spec.SpeakerConfig = &metallbv1beta1.Config{
					Affinity: &v1.Affinity{NodeAffinity: &v1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
						{Weight: 101, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{{Key: "zone",
							Operator: v1.NodeSelectorOpIn, Values: []string{"west"}}}}},
					}}},
				}
			})
			Expect(testclient.Client.Create(context.Background(), metallb)).ShouldNot(Succeed())
			metallb = metallbutils.New(OperatorNameSpace, func(m *metallbv1beta1.MetalLB) {
				m.Spec.ControllerConfig = &metallbv1beta1.Config{
					Affinity: &v1.Affinity{NodeAffinity: &v1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
						{Weight: 10, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{{Key: "zone",
							Operator: v1.NodeSelectorOpIn, Values: []string{"east"}}}}},
					}}},
				}
			})
			Expect(testclient.Client.Create(context.Background(), metallb)).Should(Succeed())
			metallbutils.DeleteAndCheck(metallb)
			metallb = metallbutils.New(OperatorNameSpace, func(m *metallbv1beta1.MetalLB) {
				m.Spec.SpeakerConfig = &metallbv1beta1.Config{
					Affinity: &v1.Affinity{NodeAffinity: &v1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
						{Weight: 100, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{{Key: "zone",
							Operator: v1.NodeSelectorOpIn, Values: []string{"west"}}}}},
					}}},
				}
			})
			Expect(testclient.Client.Create(context.Background(), metallb)).Should(Succeed())
		})
	})
})

// Gomega transformation functions for v1.Container
func envGetter(c v1.Container) []v1.EnvVar { return c.Env }
func nameGetter(c v1.Container) string     { return c.Name }
