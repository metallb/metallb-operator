package status

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
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

func TestIsMetalLBAvailable(t *testing.T) {
	tests := []struct {
		name        string
		objects     []k8sclient.Object
		expectErr   bool
		isNotReady  bool
		errContains string
	}{
		{
			name:      "all ready",
			objects:   []k8sclient.Object{newReadySpeaker(), newReadyController()},
			expectErr: false,
		},
		{
			name:      "daemonset not found",
			objects:   []k8sclient.Object{newReadyController()},
			expectErr: true,
		},
		{
			name:      "deployment not found",
			objects:   []k8sclient.Object{newReadySpeaker()},
			expectErr: true,
		},
		{
			name: "daemonset not ready",
			objects: func() []k8sclient.Object {
				ds := newReadySpeaker()
				ds.Status.NumberReady = 1
				return []k8sclient.Object{ds, newReadyController()}
			}(),
			expectErr:   true,
			isNotReady:  true,
			errContains: "speaker daemonset not ready",
		},
		{
			name: "deployment not ready",
			objects: func() []k8sclient.Object {
				deploy := newReadyController()
				deploy.Status.ReadyReplicas = 0
				return []k8sclient.Object{newReadySpeaker(), deploy}
			}(),
			expectErr:   true,
			isNotReady:  true,
			errContains: "controller deployment not ready",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			client := fake.NewClientBuilder().WithScheme(scheme()).WithObjects(tt.objects...).Build()
			err := IsMetalLBAvailable(context.Background(), client, "test-ns")
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
				if tt.isNotReady {
					g.Expect(err).To(BeAssignableToTypeOf(MetalLBResourcesNotReadyError{}))
					g.Expect(err.Error()).To(ContainSubstring(tt.errContains))
				}
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestIsDaemonSetReady(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*appsv1.DaemonSet)
		expectErr   bool
		errContains string
	}{
		{
			name:      "ready",
			mutate:    func(ds *appsv1.DaemonSet) {},
			expectErr: false,
		},
		{
			name:        "generation mismatch",
			mutate:      func(ds *appsv1.DaemonSet) { ds.Generation = 2 },
			expectErr:   true,
			errContains: "speaker daemonset status is out of date",
		},
		{
			name:        "current number scheduled mismatch",
			mutate:      func(ds *appsv1.DaemonSet) { ds.Status.CurrentNumberScheduled = 1 },
			expectErr:   true,
			errContains: "speaker daemonset not ready",
		},
		{
			name:        "ready replicas mismatch",
			mutate:      func(ds *appsv1.DaemonSet) { ds.Status.NumberReady = 1 },
			expectErr:   true,
			errContains: "speaker daemonset not ready",
		},
		{
			name:        "updated number scheduled mismatch",
			mutate:      func(ds *appsv1.DaemonSet) { ds.Status.UpdatedNumberScheduled = 1 },
			expectErr:   true,
			errContains: "speaker daemonset not ready",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			ds := newReadySpeaker()
			tt.mutate(ds)
			err := isDaemonSetReady(ds)
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(BeAssignableToTypeOf(MetalLBResourcesNotReadyError{}))
				g.Expect(err.Error()).To(ContainSubstring(tt.errContains))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestIsDeploymentReady(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*appsv1.Deployment)
		expectErr   bool
		errContains string
	}{
		{
			name:      "ready",
			mutate:    func(d *appsv1.Deployment) {},
			expectErr: false,
		},
		{
			name:        "generation mismatch",
			mutate:      func(d *appsv1.Deployment) { d.Generation = 2 },
			expectErr:   true,
			errContains: "controller deployment status is out of date",
		},
		{
			name: "replicas nil defaults to one",
			mutate: func(d *appsv1.Deployment) {
				d.Spec.Replicas = nil
				d.Status.ReadyReplicas = 1
				d.Status.UpdatedReplicas = 1
			},
			expectErr: false,
		},
		{
			name:        "ready replicas mismatch",
			mutate:      func(d *appsv1.Deployment) { d.Status.ReadyReplicas = 0 },
			expectErr:   true,
			errContains: "controller deployment not ready",
		},
		{
			name:        "updated replicas mismatch",
			mutate:      func(d *appsv1.Deployment) { d.Status.UpdatedReplicas = 0 },
			expectErr:   true,
			errContains: "controller deployment not ready",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			deploy := newReadyController()
			tt.mutate(deploy)
			err := isDeploymentReady(deploy)
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(BeAssignableToTypeOf(MetalLBResourcesNotReadyError{}))
				g.Expect(err.Error()).To(ContainSubstring(tt.errContains))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
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
