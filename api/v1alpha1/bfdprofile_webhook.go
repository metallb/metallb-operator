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
	"context"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging bfdprofile-webhook.
var bfdprofilelog = logf.Log.WithName("bfdprofile-webhook")

const (
	BFDMaxReceiveInterval      = 60000
	BFDMinReceiveInterval      = 10
	BFDMaxTransmitInterval     = 60000
	BFDMinTransmitInterval     = 10
	BFDMaxDetectMultiplier     = 255
	BFDMinDetectMultiplier     = 2
	BFDMaxEchoReceiveInterval  = 60000
	BFDMinEchoReceiveInterval  = 10
	BFDMaxEchoTransmitInterval = 60000
	BFDMinEchoTransmitInterval = 10
	BFDMaxMinimumTTL           = 254
	BFDMinMinimumTTL           = 1
)

func (bfdProfile *BFDProfile) SetupWebhookWithManager(mgr ctrl.Manager) error {
	c = mgr.GetClient()

	return ctrl.NewWebhookManagedBy(mgr).
		For(bfdProfile).
		Complete()
}

//+kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1alpha1-bfdprofile,mutating=false,failurePolicy=fail,groups=metallb.io,resources=bfdprofiles,versions=v1alpha1,name=bfdprofilevalidationwebhook.metallb.io
var _ webhook.Validator = &BFDProfile{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for BFDProfile.
func (bfdProfile *BFDProfile) ValidateCreate() error {
	bfdprofilelog.Info("validate BFDProfile creation", "name", bfdProfile.Name)

	existingBFDProfileList, err := getExistingBFDProfiles()
	if err != nil {
		return err
	}

	return bfdProfile.validateBFDProfile(true, existingBFDProfileList)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for BFDProfile.
func (bfdProfile *BFDProfile) ValidateUpdate(old runtime.Object) error {
	bfdprofilelog.Info("validate BFDProfile update", "name", bfdProfile.Name)

	existingBFDProfileList, err := getExistingBFDProfiles()
	if err != nil {
		return err
	}

	return bfdProfile.validateBFDProfile(false, existingBFDProfileList)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for BFDProfile.
func (bfdProfile *BFDProfile) ValidateDelete() error {
	bfdprofilelog.Info("validate BFDProfile deletion", "name", bfdProfile.Name)

	return nil
}

func (bfdProfile *BFDProfile) validateBFDProfile(isNewBFDProfile bool, existingBFDProfileList *BFDProfileList) error {
	for _, existingBFDProfile := range existingBFDProfileList.Items {
		if existingBFDProfile.Name == bfdProfile.Name {
			// Check that the bfdprofile isn't already defined.
			if isNewBFDProfile {
				return fmt.Errorf("duplicate definition of bfdprofile %s", bfdProfile.Name)
			} else {
				continue
			}
		}
	}

	if bfdProfile.Name == "" {
		return fmt.Errorf("missing bfdprofile name")
	}

	if bfdProfile.Spec.DetectMultiplier != nil {
		if *bfdProfile.Spec.DetectMultiplier < BFDMinDetectMultiplier ||
			*bfdProfile.Spec.DetectMultiplier > BFDMaxDetectMultiplier {
			return fmt.Errorf("invalid detect multiplier value")
		}
	}
	if bfdProfile.Spec.ReceiveInterval != nil {
		if *bfdProfile.Spec.ReceiveInterval < BFDMinReceiveInterval ||
			*bfdProfile.Spec.ReceiveInterval > BFDMaxReceiveInterval {
			return fmt.Errorf("invalid receive interval value")
		}
	}
	if bfdProfile.Spec.TransmitInterval != nil {
		if *bfdProfile.Spec.TransmitInterval < BFDMinTransmitInterval ||
			*bfdProfile.Spec.TransmitInterval > BFDMaxTransmitInterval {
			return fmt.Errorf("invalid transmit interval value")
		}
	}
	if bfdProfile.Spec.MinimumTTL != nil {
		if *bfdProfile.Spec.MinimumTTL < BFDMinMinimumTTL ||
			*bfdProfile.Spec.MinimumTTL > BFDMaxMinimumTTL {
			return fmt.Errorf("invalid minimum ttl value")
		}
	}
	if bfdProfile.Spec.EchoReceiveInterval != nil {
		echoReceiveInterval, err := strconv.Atoi(*bfdProfile.Spec.EchoReceiveInterval)
		if err != nil {
			if *bfdProfile.Spec.EchoReceiveInterval != "disabled" {
				return fmt.Errorf("invalid echo receive interval value")
			}
		}
		if echoReceiveInterval < BFDMinEchoReceiveInterval ||
			echoReceiveInterval > BFDMaxEchoReceiveInterval {
			return fmt.Errorf("invalid echo receive interval value")
		}
	}
	if bfdProfile.Spec.EchoTransmitInterval != nil {
		if *bfdProfile.Spec.TransmitInterval < BFDMinEchoTransmitInterval ||
			*bfdProfile.Spec.TransmitInterval > BFDMaxEchoTransmitInterval {
			return fmt.Errorf("invalid echo transmit interval value")
		}
	}
	return nil
}

func getExistingBFDProfiles() (*BFDProfileList, error) {
	existingBFDProfileList := &BFDProfileList{}
	err := c.List(context.Background(), existingBFDProfileList)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get existing bfdprofile objects")
	}
	return existingBFDProfileList, nil
}
