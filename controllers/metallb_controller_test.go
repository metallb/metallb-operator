package controllers

import (
	"context"
	"os"
	"time"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/test/consts"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("MetalLB Controller", func() {
	Context("syncMetalLB", func() {
		metallb := &metallbv1beta1.MetalLB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "metallb",
				Namespace: MetalLBTestNameSpace,
			},
		}
		AfterEach(func() {
			err := k8sClient.Delete(context.Background(), metallb)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					Fail(err.Error())
				}
			}
			err = cleanTestNamespace()
			Expect(err).ToNot(HaveOccurred())
		})
		It("Should create manifests with images and namespace overriden", func() {
			speakerImage := "test-speaker:latest"
			controllerImage := "test-controller:latest"
			By("Setting the environment variables")
			Expect(os.Setenv("SPEAKER_IMAGE", speakerImage)).To(Succeed())
			Expect(os.Setenv("CONTROLLER_IMAGE", controllerImage)).To(Succeed())
			Expect(os.Setenv("WATCH_NAMESPACE", MetalLBTestNameSpace)).To(Succeed())

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
			Expect(controllerDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal(controllerImage))

			speakerDaemonSet := &appsv1.DaemonSet{}
			Eventually(func() error {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: consts.MetalLBDaemonsetName, Namespace: MetalLBTestNameSpace}, speakerDaemonSet)
				return err
			}, 2*time.Second, 200*time.Millisecond).ShouldNot((HaveOccurred()))
			Expect(speakerDaemonSet).NotTo(BeZero())
			Expect(len(speakerDaemonSet.Spec.Template.Spec.Containers)).To(BeNumerically(">", 0))
			Expect(speakerDaemonSet.Spec.Template.Spec.Containers[0].Image).To(Equal(speakerImage))
		})
	})
})

func cleanTestNamespace() error {
	err := k8sClient.DeleteAllOf(context.Background(), &appsv1.Deployment{}, client.InNamespace(MetalLBTestNameSpace))
	if err != nil {
		return err
	}
	err = k8sClient.DeleteAllOf(context.Background(), &appsv1.DaemonSet{}, client.InNamespace(MetalLBTestNameSpace))
	return err
}
