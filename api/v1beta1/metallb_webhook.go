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
	"errors"
	"fmt"
	"net"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var ExternalFRRK8sNamespace string

func (metallb *MetalLB) SetupWebhookWithManager(mgr ctrl.Manager, externalFRRK8sNamespace string) error {
	ExternalFRRK8sNamespace = externalFRRK8sNamespace
	return ctrl.NewWebhookManagedBy(mgr).
		For(metallb).
		Complete()
}

//+kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1beta1-metallb,mutating=false,failurePolicy=fail,groups=metallb.io,resources=metallbs,versions=v1beta1,name=metallbvalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for MetalLB.
func (metallb *MetalLB) ValidateCreate() (admission.Warnings, error) {
	if err := metallb.Validate(); err != nil {
		return admission.Warnings{}, err
	}

	return admission.Warnings{}, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for MetalLB.
func (metallb *MetalLB) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	if err := metallb.Validate(); err != nil {
		return admission.Warnings{}, err
	}

	return admission.Warnings{}, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for MetalLB.
func (metallb *MetalLB) ValidateDelete() (admission.Warnings, error) {
	return admission.Warnings{}, nil
}

func (metallb *MetalLB) Validate() error {
	for _, ct := range metallb.Spec.ControllerTolerations {
		if ct.TolerationSeconds != nil && *ct.TolerationSeconds > 0 && ct.Effect != v1.TaintEffectNoExecute {
			return errors.New("ControllerToleration effect must be NoExecute when tolerationSeconds is set")
		}
	}
	for _, ct := range metallb.Spec.SpeakerTolerations {
		if ct.TolerationSeconds != nil && *ct.TolerationSeconds > 0 && ct.Effect != v1.TaintEffectNoExecute {
			return errors.New("SpeakerToleration effect must be NoExecute when tolerationSeconds is set")
		}
	}
	if metallb.Spec.SpeakerConfig != nil && metallb.Spec.SpeakerConfig.Affinity != nil &&
		metallb.Spec.SpeakerConfig.Affinity.NodeAffinity != nil {
		for _, pst := range metallb.Spec.SpeakerConfig.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			if pst.Weight < 1 || pst.Weight > 100 {
				return errors.New("SpeakerConfig NodeAffinity set with invalid weight for preferred scheduling term")
			}
		}
	}
	if metallb.Spec.SpeakerConfig != nil && metallb.Spec.SpeakerConfig.Affinity != nil &&
		metallb.Spec.SpeakerConfig.Affinity.PodAffinity != nil {
		for _, pst := range metallb.Spec.SpeakerConfig.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			if pst.Weight < 1 || pst.Weight > 100 {
				return errors.New("SpeakerConfig PodAffinity set with invalid weight for preferred scheduling term")
			}
		}
	}
	if metallb.Spec.ControllerConfig != nil && metallb.Spec.ControllerConfig.Affinity != nil &&
		metallb.Spec.ControllerConfig.Affinity.NodeAffinity != nil {
		for _, pst := range metallb.Spec.ControllerConfig.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			if pst.Weight < 1 || pst.Weight > 100 {
				return errors.New("ControllerConfig NodeAffinity set with invalid weight for preferred scheduling term")
			}
		}
	}
	if metallb.Spec.ControllerConfig != nil && metallb.Spec.ControllerConfig.Affinity != nil &&
		metallb.Spec.ControllerConfig.Affinity.PodAffinity != nil {
		for _, pst := range metallb.Spec.ControllerConfig.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			if pst.Weight < 1 || pst.Weight > 100 {
				return errors.New("ControllerConfig PodAffinity set with invalid weight for preferred scheduling term")
			}
		}
	}

	if metallb.Spec.BGPBackend != "" &&
		metallb.Spec.BGPBackend != NativeMode &&
		metallb.Spec.BGPBackend != FRRK8sMode &&
		metallb.Spec.BGPBackend != FRRK8sExternalMode &&
		metallb.Spec.BGPBackend != FRRMode {
		return errors.New("Invalid BGP Backend, must be one of native, frr, frr-k8s")
	}

	if err := validateFRRK8sConfig(metallb.Spec); err != nil {
		return err
	}
	return nil
}

func validateFRRK8sConfig(spec MetalLBSpec) error {
	config := spec.FRRK8SConfig
	if spec.BGPBackend == FRRK8sExternalMode &&
		ExternalFRRK8sNamespace == "" && (config == nil || config.Namespace == "") {
		return errors.New("bgp backend: frrk8s external and no default or user provided namespace")
	}

	if config == nil {
		return nil
	}
	for _, cidr := range config.AlwaysBlock {
		_, _, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			return fmt.Errorf("invalid CIDR %s in AlwaysBlock", cidr)
		}
	}
	return nil
}
