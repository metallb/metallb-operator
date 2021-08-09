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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BGPCommunitySpec defines the desired state of BGPCommunity
type BGPCommunitySpec struct {
	BGPCommunity map[string]string `json:"bgpCommunities,omitempty" yaml:"bgp-communities,omitempty"`
}

// BGPCommunityStatus defines the observed state of BGPCommunity
type BGPCommunityStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// BGPCommunity is the Schema for the bgpcommunities API
type BGPCommunity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BGPCommunitySpec   `json:"spec,omitempty"`
	Status BGPCommunityStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BGPCommunityList contains a list of BGPCommunity
type BGPCommunityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BGPCommunity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BGPCommunity{}, &BGPCommunityList{})
}
