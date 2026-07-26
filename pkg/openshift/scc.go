package openshift

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const SpeakerSCCName = "metallb-speaker"

func SpeakerSCC() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "security.openshift.io/v1",
			"kind":       "SecurityContextConstraints",
			"metadata": map[string]interface{}{
				"name": SpeakerSCCName,
				"annotations": map[string]interface{}{
					"kubernetes.io/description": "Custom SCC for MetalLB speaker.",
				},
			},
			"allowHostDirVolumePlugin": false,
			"allowHostIPC":             false,
			"allowHostNetwork":         true,
			"allowHostPID":             false,
			"allowHostPorts":           true,
			"allowPrivilegeEscalation": false,
			"allowPrivilegedContainer": false,
			"allowedCapabilities": []interface{}{
				"NET_RAW",
			},
			"defaultAddCapabilities": []interface{}{},
			"fsGroup": map[string]interface{}{
				"type": "RunAsAny",
			},
			"readOnlyRootFilesystem": false,
			"requiredDropCapabilities": []interface{}{
				"ALL",
			},
			"runAsUser": map[string]interface{}{
				"type": "RunAsAny",
			},
			"seLinuxContext": map[string]interface{}{
				"type": "RunAsAny",
			},
			"supplementalGroups": map[string]interface{}{
				"type": "RunAsAny",
			},
			"volumes": []interface{}{
				"configMap",
				"downwardAPI",
				"emptyDir",
				"projected",
				"secret",
			},
			"users":    []interface{}{},
			"groups":   []interface{}{},
			"priority": nil,
		},
	}
}
