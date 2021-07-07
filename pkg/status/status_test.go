package status

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func validateConditionTypes(g *GomegaWithT, conditions []metav1.Condition) {
	g.Expect(conditions[0].Type).To(Equal(ConditionAvailable))
	g.Expect(conditions[1].Type).To(Equal(ConditionUpgradeable))
	g.Expect(conditions[2].Type).To(Equal(ConditionProgressing))
	g.Expect(conditions[3].Type).To(Equal(ConditionDegraded))
}
