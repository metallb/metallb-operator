package consts

import (
	"time"
)

const (
	// MetalLBOperatorDeploymentName contains the name of the MetalLB Operator deployment
	MetalLBOperatorDeploymentName = "metallb-operator-controller-manager"
	// MetalLBOperatorDeploymentLabel contains the label of the MetalLB Operator deployment
	MetalLBOperatorDeploymentLabel = "controller-manager"
	// MetalLBOperatorCRDName contains the name of the MetalLB Operator CRD
	MetalLBOperatorCRDName = "metallbs.metallb.io"
	// MetalLBCRFile contains the MetalLB custom resource deployment
	MetalLBCRFile = "metallb.yaml"
	// MetalLBDeploymentName contains the name of the MetalLB deployment
	MetalLBDeploymentName = "controller"
	// MetalLBDaemonsetName contains the name of the MetalLB daemonset
	MetalLBDaemonsetName = "speaker"
	// MetalLBConfigMapName contains created configmap
	MetalLBConfigMapName = "config"
	// FRRK8SDaemonsetName contains the name of the frr-k8s daemonset
	FRRK8SDaemonsetName = "frr-k8s"
	// FRRK8SStatusCleanerDeploymentName contains the name of the frr-k8s statuscleaner deployment
	FRRK8SStatusCleanerDeploymentName = "statuscleaner"
	// FRRK8SDaemonsetLabelSelector contains the label selector of the frr-k8s daemonset
	FRRK8SDaemonsetLabelSelector = "app.kubernetes.io/component=frr-k8s"
	// FRRK8SStatusCleanerLabelSelector contains the label selector of the frr-k8s statuscleaner deployment
	FRRK8SStatusCleanerLabelSelector = "component=statuscleaner"
	// DefaultOperatorNameSpace is the default operator namespace
	DefaultOperatorNameSpace = "metallb-system"
	// LogsExtractDuration represents how much in the past to fetch the logs from
	LogsExtractDuration = 10 * time.Minute
)
