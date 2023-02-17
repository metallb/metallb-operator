package status

import (
	"context"
	"time"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// hello
type MetalLBResourcesNotReadyError struct {
	Message string
}

func (e MetalLBResourcesNotReadyError) Error() string { return e.Message }

func (e MetalLBResourcesNotReadyError) Is(target error) bool {
	_, ok := target.(*MetalLBResourcesNotReadyError)
	return ok
}

type DeploymentNotReadyError struct{}

const (
	ConditionAvailable   = "Available"
	ConditionProgressing = "Progressing"
	ConditionDegraded    = "Degraded"
	ConditionUpgradeable = "Upgradeable"
)

func Update(ctx context.Context, client k8sclient.Client, metallb *metallbv1beta1.MetalLB, condition string, reason string, message string) error {
	conditions := getConditions(condition, reason, message)
	if equality.Semantic.DeepEqual(conditions, metallb.Status.Conditions) {
		return nil
	}
	metallb.Status.Conditions = getConditions(condition, reason, message)

	if err := client.Status().Update(ctx, metallb); err != nil {
		return errors.Wrapf(err, "could not update status for object %+v", metallb)
	}
	return nil
}

func getConditions(condition string, reason string, message string) []metav1.Condition {
	conditions := getBaseConditions()
	switch condition {
	case ConditionAvailable:
		conditions[0].Status = metav1.ConditionTrue
		conditions[1].Status = metav1.ConditionTrue
	case ConditionProgressing:
		conditions[2].Status = metav1.ConditionTrue
		conditions[2].Reason = reason
		conditions[2].Message = message
	case ConditionDegraded:
		conditions[3].Status = metav1.ConditionTrue
		conditions[3].Reason = reason
		conditions[3].Message = message
	}
	return conditions
}

func getBaseConditions() []metav1.Condition {
	now := time.Now()
	return []metav1.Condition{
		{
			Type:               ConditionAvailable,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
			Reason:             ConditionAvailable,
		},
		{
			Type:               ConditionUpgradeable,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
			Reason:             ConditionUpgradeable,
		},
		{
			Type:               ConditionProgressing,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
			Reason:             ConditionProgressing,
		},
		{
			Type:               ConditionDegraded,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Time{Time: now},
			Reason:             ConditionDegraded,
		},
	}
}

func IsMetalLBAvailable(ctx context.Context, client k8sclient.Client, namespace string) error {

	ds := &appsv1.DaemonSet{}
	err := client.Get(ctx, types.NamespacedName{Name: "speaker", Namespace: namespace}, ds)
	if err != nil {
		return err
	}
	if ds.Status.DesiredNumberScheduled != ds.Status.CurrentNumberScheduled {
		return MetalLBResourcesNotReadyError{Message: "MetalLB speaker daemonset not ready"}
	}
	deployment := &appsv1.Deployment{}
	err = client.Get(ctx, types.NamespacedName{Name: "controller", Namespace: namespace}, deployment)
	if err != nil {
		return err
	}
	if deployment.Status.ReadyReplicas != *deployment.Spec.Replicas {
		return MetalLBResourcesNotReadyError{Message: "MetalLB controller deployment not ready"}
	}
	return nil
}
