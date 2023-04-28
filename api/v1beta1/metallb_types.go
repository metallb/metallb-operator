/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type MetalLBLogLevel string

// These are valid logging level for MetalLB components.
const (
	LogLevelAll   MetalLBLogLevel = "all"
	LogLevelDebug MetalLBLogLevel = "debug"
	LogLevelInfo  MetalLBLogLevel = "info"
	LogLevelWarn  MetalLBLogLevel = "warn"
	LogLevelError MetalLBLogLevel = "error"
	LogLevelNone  MetalLBLogLevel = "none"
)

// MetalLBSpec defines the desired state of MetalLB
type MetalLBSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of MetalLB. Edit MetalLB_types.go to remove/update
	MetalLBImage string `json:"image,omitempty"`

	// node selector applied to MetalLB speaker daemonset.
	// +optional
	SpeakerNodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// tolerations is a list of tolerations applied to MetalLB speaker
	// daemonset.
	// +optional
	SpeakerTolerations []corev1.Toleration `json:"speakerTolerations,omitempty"`

	// Define the verbosity of the controller and the speaker logging.
	// Allowed values are: all, debug, info, warn, error, none. (default: info)
	// +optional
	// +kubebuilder:validation:Enum=all;debug;info;warn;error;none
	LogLevel MetalLBLogLevel `json:"logLevel,omitempty"`

	// The loadBalancerClass spec attribute that the MetalLB controller should
	// be watching for. must be a label-style identifier, with an optional
	// prefix such as "internal-vip" or "example.com/internal-vip". Unprefixed
	// names are reserved for end-users.
	// +optional
	// +kubebuilder:validation:Pattern=`^([a-z0-9A-Z]([\w.\-]*[a-z0-9A-Z])?/)?[a-z0-9A-Z]([\w.\-]*[a-z0-9A-Z])?$`
	LoadBalancerClass string `json:"loadBalancerClass,omitempty"`

	// node selector applied to MetalLB controller deployment.
	// +optional
	ControllerNodeSelector map[string]string `json:"controllerNodeSelector,omitempty"`

	// tolerations is a list of tolerations applied to MetalLB controller
	// deployment.
	// +optional
	ControllerTolerations []corev1.Toleration `json:"controllerTolerations,omitempty"`

	// additional configs to be applied on MetalLB Controller deployment.
	// +optional
	ControllerConfig *Config `json:"controllerConfig,omitempty"`

	// additional configs to be applied on MetalLB Speaker daemonset.
	// +optional
	SpeakerConfig *Config `json:"speakerConfig,omitempty"`
}

type Config struct {
	// Define priority class name
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// Define container runtime configuration class
	// +optional
	RuntimeClassName string `json:"runtimeClassName,omitempty"`

	// If specified, the pod's scheduling constraints
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Annotations to be applied for MetalLB Operator
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Resource Requirements to be applied for containers which gets deployed
	// via MetalLB Operator
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// MetalLBStatus defines the observed state of MetalLB
type MetalLBStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Conditions show the current state of the MetalLB Operator
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MetalLB is the Schema for the metallbs API
type MetalLB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MetalLBSpec   `json:"spec,omitempty"`
	Status MetalLBStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MetalLBList contains a list of MetalLB
type MetalLBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetalLB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MetalLB{}, &MetalLBList{})
}
