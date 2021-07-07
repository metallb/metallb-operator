package consts

const (
	// MetallbNameSpace contains the name of the MetalLB operator  namespace
	MetallbNameSpace = "metallb-system"
	// MetallbOperatorDeploymentName contains the name of the MetalLB operator deployment
	MetallbOperatorDeploymentName = "metallboperator-controller-manager"
	// MetallbOperatorDeploymentLabel contains the label of the MetalLB operator deployment
	MetallbOperatorDeploymentLabel = "controller-manager"
	// MetallbOperatorCRDName contains the name of the MetalLB operator CRD
	MetallbOperatorCRDName = "metallbs.metallb.io"
	// MetallbCRFile contains the Metallb custom resource deployment
	MetallbCRFile = "metallb.yaml"
	// MetallbDeploymentName contains the name of the MetalLB deployment
	MetallbDeploymentName = "controller"
	// MetallbDaemonsetName contains the name of the MetalLB daemonset
	MetallbDaemonsetName = "speaker"
	// MetallbAddressPoolCRDName contains the name of MetallB AddressPool CRD
	MetallbAddressPoolCRDName = "addresspools.metallb.io"
	// MetallbConfigMapName contains created configmap
	MetallbConfigMapName = "config"
)
