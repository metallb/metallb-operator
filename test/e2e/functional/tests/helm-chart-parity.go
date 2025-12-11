package tests

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metallboperatorv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/test/consts"
	testclient "github.com/metallb/metallb-operator/test/e2e/client"
	metallbutils "github.com/metallb/metallb-operator/test/e2e/metallb"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	goclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	helmChartKeyExample       = "example"
	helmChartKeyMyClass       = "myclass"
	helmChartKeyOperator      = "Exists"
	helmChartKeyEffect        = "NoExecute"
	helmChartDemoController   = "demo-controller"
	helmChartControllerTest   = "controller-test"
	helmChartDemoSpeaker      = "demo-speaker"
	helmChartSpeakerTest      = "speaker-test"
	helmChartHighPriority     = "high-priority"
	kubernetesHostnameLabel   = "kubernetes.io/hostname"
	helmChartValidationPeriod = 30 * time.Second
)

var helmChartToleration = corev1.Toleration{
	Key:      helmChartKeyExample,
	Operator: corev1.TolerationOperator(helmChartKeyOperator),
	Effect:   corev1.TaintEffect(helmChartKeyEffect),
}

var _ = Describe("MetalLB Helm chart parity", func() {
	AfterEach(func() {
		By("Cleaning up MetalLB configuration")
		metallb, err := metallbutils.Get(OperatorNameSpace, UseMetallbResourcesFromFile)
		Expect(err).ToNot(HaveOccurred())
		metallbutils.DeleteAndCheck(metallb)
	})

	It("deploys MetalLB with Helm chart parameters", func() {
		By("Creating MetalLB with Helm chart parity configuration")
		createMetalLBHelmChartNoUpdate()

		By("Validating MetalLB workloads expose Helm chart configuration")
		Expect(validateHelmChartDeployment()).To(Succeed())
	})

	It("updates Helm chart parameters on an existing deployment", func() {
		By("Ensuring a baseline MetalLB deployment is present")
		ensureMetalLBExists()

		By("Updating Helm chart fields")
		updateMetalLBHelmChart()

		By("Validating MetalLB workloads expose Helm chart configuration")
		Eventually(validateHelmChartDeployment, time.Minute, 5*time.Second).Should(Succeed())
	})
})

func updateMetalLBHelmChart() {
	By("Updating MetalLB custom resource with Helm chart values")

	metallb, err := getMetallb()
	Expect(err).ToNot(HaveOccurred(), "failed to retrieve MetalLB custom resource")

	metallbConfig := defineMetalLBHelmChart(metallb)
	Expect(testclient.Client.Update(context.Background(), metallbConfig)).To(Succeed(), "failed to update MetalLB resource")
}

func createMetalLBHelmChartNoUpdate() {
	metallb, err := metallbutils.Get(OperatorNameSpace, UseMetallbResourcesFromFile)
	Expect(err).ToNot(HaveOccurred())

	err = testclient.Client.Get(context.Background(), goclient.ObjectKey{Namespace: metallb.Namespace, Name: metallb.Name}, metallb)
	if apierrors.IsNotFound(err) {
		metallb = defineMetalLBHelmChart(metallb)
		Expect(testclient.Client.Create(context.Background(), metallb)).To(Succeed(), "failed to create MetalLB resource")
		return
	}
	Expect(err).ToNot(HaveOccurred(), "failed to retrieve MetalLB custom resource")

	metallb = defineMetalLBHelmChart(metallb)
	Expect(testclient.Client.Update(context.Background(), metallb)).To(Succeed(), "failed to update MetalLB resource")
}

func ensureMetalLBExists() {
	metallb, err := metallbutils.Get(OperatorNameSpace, UseMetallbResourcesFromFile)
	Expect(err).ToNot(HaveOccurred())

	err = testclient.Client.Get(context.Background(), goclient.ObjectKey{Namespace: metallb.Namespace, Name: metallb.Name}, metallb)
	if apierrors.IsNotFound(err) {
		Expect(testclient.Client.Create(context.Background(), metallb)).To(Succeed(), "failed to create baseline MetalLB resource")
		return
	}
	Expect(err).ToNot(HaveOccurred())
}

func validateHelmChartDeployment() error {
	if err := validateHelmChartControllerDeployment(); err != nil {
		return err
	}

	return validateHelmChartSpeakerDaemonSet()
}

func validateHelmChartControllerDeployment() error {
	Eventually(func() error {
		_, err := testclient.Client.Deployments(OperatorNameSpace).Get(context.Background(), consts.MetalLBDeploymentName, metav1.GetOptions{})
		return err
	}, helmChartValidationPeriod, metallbutils.Interval).ShouldNot(HaveOccurred(), "failed waiting for MetalLB controller deployment")

	controlDep, err := testclient.Client.Deployments(OperatorNameSpace).Get(context.Background(), consts.MetalLBDeploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if controlDep.Spec.Template.Spec.PriorityClassName != helmChartKeyExample {
		return fmt.Errorf("controller PriorityClassName mismatch")
	}

	if controlDep.Spec.Template.Spec.RuntimeClassName == nil || *controlDep.Spec.Template.Spec.RuntimeClassName != helmChartKeyMyClass {
		return fmt.Errorf("controller RuntimeClassName mismatch")
	}

	if controlDep.Spec.Template.Annotations[consts.MetalLBDeploymentName] != helmChartDemoController {
		return fmt.Errorf("controller annotation mismatch")
	}

	if err := validateAffinity(controlDep.Spec.Template.Spec.Affinity, helmChartControllerTest); err != nil {
		return err
	}

	if err := validateTolerations(controlDep.Spec.Template.Spec.Tolerations); err != nil {
		return err
	}

	return nil
}

func validateHelmChartSpeakerDaemonSet() error {
	Eventually(func() error {
		_, err := testclient.Client.DaemonSets(OperatorNameSpace).Get(context.Background(), consts.MetalLBDaemonsetName, metav1.GetOptions{})
		return err
	}, helmChartValidationPeriod, metallbutils.Interval).ShouldNot(HaveOccurred(), "failed waiting for MetalLB speaker daemonset")

	speakerDep, err := testclient.Client.DaemonSets(OperatorNameSpace).Get(context.Background(), consts.MetalLBDaemonsetName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if speakerDep.Spec.Template.Spec.PriorityClassName != helmChartHighPriority {
		return fmt.Errorf("speaker PriorityClassName mismatch")
	}

	if speakerDep.Spec.Template.Spec.RuntimeClassName == nil || *speakerDep.Spec.Template.Spec.RuntimeClassName != helmChartKeyMyClass {
		return fmt.Errorf("speaker RuntimeClassName mismatch")
	}

	if speakerDep.Spec.Template.Annotations[consts.MetalLBDeploymentName] != helmChartDemoSpeaker {
		return fmt.Errorf("speaker annotation mismatch")
	}

	if err := validateAffinity(speakerDep.Spec.Template.Spec.Affinity, helmChartSpeakerTest); err != nil {
		return err
	}

	if err := validateTolerations(speakerDep.Spec.Template.Spec.Tolerations); err != nil {
		return err
	}

	return nil
}

func validateAffinity(affinity *corev1.Affinity, expectedComponent string) error {
	if affinity == nil || affinity.PodAffinity == nil {
		return fmt.Errorf("missing pod affinity configuration")
	}

	terms := affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	if len(terms) == 0 || terms[0].LabelSelector == nil {
		return fmt.Errorf("missing pod affinity terms")
	}

	if terms[0].LabelSelector.MatchLabels["component"] != expectedComponent {
		return fmt.Errorf("pod affinity matchLabels mismatch")
	}

	return nil
}

func validateTolerations(tolerations []corev1.Toleration) error {
	if len(tolerations) == 0 {
		return fmt.Errorf("missing tolerations")
	}

	for _, tol := range tolerations {
		if tol != helmChartToleration {
			continue
		}

		return nil
	}

	return fmt.Errorf("expected toleration key=%s operator=%s effect=%s not found",
		helmChartKeyExample, helmChartKeyOperator, helmChartKeyEffect)
}

func defineMetalLBHelmChart(metallb *metallboperatorv1beta1.MetalLB) *metallboperatorv1beta1.MetalLB {
	controllerConfig := &metallboperatorv1beta1.Config{
		PriorityClassName: helmChartKeyExample,
		RuntimeClassName:  helmChartKeyMyClass,
		Annotations: map[string]string{
			consts.MetalLBDeploymentName: helmChartDemoController,
		},
		Affinity: &corev1.Affinity{
			PodAffinity: &corev1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"component": helmChartControllerTest,
							},
						},
						TopologyKey: kubernetesHostnameLabel,
					},
				},
			},
		},
	}

	speakerConfig := &metallboperatorv1beta1.Config{
		PriorityClassName: helmChartHighPriority,
		RuntimeClassName:  helmChartKeyMyClass,
		Annotations: map[string]string{
			consts.MetalLBDeploymentName: helmChartDemoSpeaker,
		},
		Affinity: &corev1.Affinity{
			PodAffinity: &corev1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"component": helmChartSpeakerTest,
							},
						},
						TopologyKey: kubernetesHostnameLabel,
					},
				},
			},
		},
	}

	metallb.Spec.ControllerConfig = controllerConfig
	metallb.Spec.ControllerTolerations = []corev1.Toleration{helmChartToleration}

	metallb.Spec.SpeakerConfig = speakerConfig
	metallb.Spec.SpeakerTolerations = []corev1.Toleration{helmChartToleration}

	return metallb
}

func getMetallb() (*metallboperatorv1beta1.MetalLB, error) {
	metallb, err := metallbutils.Get(OperatorNameSpace, UseMetallbResourcesFromFile)
	if err != nil {
		return nil, err
	}

	err = testclient.Client.Get(context.Background(), goclient.ObjectKey{Namespace: metallb.Namespace, Name: metallb.Name}, metallb)
	return metallb, err
}
