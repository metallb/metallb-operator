package status

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetConditionsAvailable(t *testing.T) {
	g := NewGomegaWithT(t)
	conditions := getConditions(ConditionAvailable, "testReason", "testMessage")
	validateUnsetConditions(g, conditions, []int{2, 3})
	validateConditionTypes(g, conditions)
	g.Expect(conditions[0].Status).To(Equal(metav1.ConditionTrue))
	g.Expect(conditions[1].Status).To(Equal(metav1.ConditionTrue))
}

func TestGetConditionsProgressing(t *testing.T) {
	g := NewGomegaWithT(t)
	conditions := getConditions(ConditionProgressing, "testReason", "testMessage")
	validateUnsetConditions(g, conditions, []int{0, 1, 3})
	validateConditionTypes(g, conditions)
	g.Expect(conditions[2].Status).To(Equal(metav1.ConditionTrue))
	g.Expect(conditions[2].Message).To(Equal("testMessage"))
	g.Expect(conditions[2].Reason).To(Equal("testReason"))
}

func TestGetConditionsDegraded(t *testing.T) {
	g := NewGomegaWithT(t)
	conditions := getConditions(ConditionDegraded, "testReason", "testMessage")
	validateUnsetConditions(g, conditions, []int{0, 1, 2})
	validateConditionTypes(g, conditions)
	g.Expect(conditions[3].Status).To(Equal(metav1.ConditionTrue))
	g.Expect(conditions[3].Message).To(Equal("testMessage"))
	g.Expect(conditions[3].Reason).To(Equal("testReason"))
}

func validateUnsetConditions(g *GomegaWithT, conditions []metav1.Condition, indexes []int) {
	for _, index := range indexes {
		g.Expect(conditions[index].Status).To(Equal(metav1.ConditionFalse))
		g.Expect(conditions[index].Message).To(Equal(""))
	}
}

func newReadySpeaker() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "speaker", Namespace: "test-ns", Generation: 1},
		Status: appsv1.DaemonSetStatus{
			ObservedGeneration:     1,
			DesiredNumberScheduled: 3,
			CurrentNumberScheduled: 3,
			NumberReady:            3,
			UpdatedNumberScheduled: 3,
		},
	}
}

func newReadyController() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "controller", Namespace: "test-ns", Generation: 1},
		Spec:       appsv1.DeploymentSpec{Replicas: ptr.To(int32(2))},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 1,
			ReadyReplicas:      2,
			UpdatedReplicas:    2,
		},
	}
}

func TestIsMetalLBAvailable_AllReady(t *testing.T) {
	g := NewGomegaWithT(t)
	client := fake.NewClientBuilder().WithScheme(scheme()).WithObjects(newReadySpeaker(), newReadyController()).Build()
	err := IsMetalLBAvailable(context.Background(), client, "test-ns")
	g.Expect(err).ToNot(HaveOccurred())
}

func TestIsMetalLBAvailable_DaemonSetNotFound(t *testing.T) {
	g := NewGomegaWithT(t)
	client := fake.NewClientBuilder().WithScheme(scheme()).WithObjects(newReadyController()).Build()
	err := IsMetalLBAvailable(context.Background(), client, "test-ns")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).ToNot(BeAssignableToTypeOf(MetalLBResourcesNotReadyError{}))
}

func TestIsMetalLBAvailable_DeploymentNotFound(t *testing.T) {
	g := NewGomegaWithT(t)
	client := fake.NewClientBuilder().WithScheme(scheme()).WithObjects(newReadySpeaker()).Build()
	err := IsMetalLBAvailable(context.Background(), client, "test-ns")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).ToNot(BeAssignableToTypeOf(MetalLBResourcesNotReadyError{}))
}

func TestIsMetalLBAvailable_DeploymentGenerationMismatch(t *testing.T) {
	g := NewGomegaWithT(t)
	deploy := newReadyController()
	deploy.Generation = 2
	client := fake.NewClientBuilder().WithScheme(scheme()).WithObjects(newReadySpeaker(), deploy).Build()
	err := IsMetalLBAvailable(context.Background(), client, "test-ns")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(BeAssignableToTypeOf(MetalLBResourcesNotReadyError{}))
	g.Expect(err.Error()).To(ContainSubstring("controller deployment status is out of date"))
}

func TestIsMetalLBAvailable_DaemonSetGenerationMismatch(t *testing.T) {
	g := NewGomegaWithT(t)
	ds := newReadySpeaker()
	ds.Generation = 2
	client := fake.NewClientBuilder().WithScheme(scheme()).WithObjects(ds, newReadyController()).Build()
	err := IsMetalLBAvailable(context.Background(), client, "test-ns")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(BeAssignableToTypeOf(MetalLBResourcesNotReadyError{}))
	g.Expect(err.Error()).To(ContainSubstring("speaker daemonset status is out of date"))
}

func TestIsMetalLBAvailable_DaemonSetUpdatedNumberScheduledMismatch(t *testing.T) {
	g := NewGomegaWithT(t)
	ds := newReadySpeaker()
	ds.Status.UpdatedNumberScheduled = 1
	client := fake.NewClientBuilder().WithScheme(scheme()).WithObjects(ds, newReadyController()).Build()
	err := IsMetalLBAvailable(context.Background(), client, "test-ns")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(BeAssignableToTypeOf(MetalLBResourcesNotReadyError{}))
	g.Expect(err.Error()).To(ContainSubstring("speaker daemonset not ready"))
}

func TestIsMetalLBAvailable_DeploymentReplicasNilDefaultsToOne(t *testing.T) {
	g := NewGomegaWithT(t)
	deploy := newReadyController()
	deploy.Spec.Replicas = nil
	deploy.Status.ReadyReplicas = 1
	deploy.Status.UpdatedReplicas = 1
	client := fake.NewClientBuilder().WithScheme(scheme()).WithObjects(newReadySpeaker(), deploy).Build()
	err := IsMetalLBAvailable(context.Background(), client, "test-ns")
	g.Expect(err).ToNot(HaveOccurred())
}

func TestIsMetalLBAvailable_DeploymentUpdatedReplicasMismatch(t *testing.T) {
	g := NewGomegaWithT(t)
	deploy := newReadyController()
	deploy.Status.UpdatedReplicas = 0
	client := fake.NewClientBuilder().WithScheme(scheme()).WithObjects(newReadySpeaker(), deploy).Build()
	err := IsMetalLBAvailable(context.Background(), client, "test-ns")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(BeAssignableToTypeOf(MetalLBResourcesNotReadyError{}))
	g.Expect(err.Error()).To(ContainSubstring("controller deployment not ready"))
}

func scheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = appsv1.AddToScheme(s)
	return s
}

func validateConditionTypes(g *GomegaWithT, conditions []metav1.Condition) {
	g.Expect(conditions[0].Type).To(Equal(ConditionAvailable))
	g.Expect(conditions[1].Type).To(Equal(ConditionUpgradeable))
	g.Expect(conditions[2].Type).To(Equal(ConditionProgressing))
	g.Expect(conditions[3].Type).To(Equal(ConditionDegraded))
}
