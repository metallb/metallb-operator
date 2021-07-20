package consts

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
	// MetalLBAddressPoolCRDName contains the name of MetallB AddressPool CRD
	MetalLBAddressPoolCRDName = "addresspools.metallb.io"
	// MetalLBConfigMapName contains created configmap
	MetalLBConfigMapName = "config"
)
