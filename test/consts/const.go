package consts

import (
	"time"

	"github.com/metallb/metallb-operator/pkg/apply"
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
	MetalLBConfigMapName = apply.MetalLBConfigMap
	// DefaultOperatorNameSpace is the default operator namespace
	DefaultOperatorNameSpace = "metallb-system"
	// LogsExtractDuration represents how much in the past to fetch the logs from
	LogsExtractDuration = 10 * time.Minute
)
