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
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"net"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging bgppeer-webhook
var bgppeerlog = logf.Log.WithName("bgppeer-webhook")

func (bgpPeer *BGPPeer) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(bgpPeer).
		Complete()
}

//+kubebuilder:webhook:verbs=create;update,path=/validate-metallb-io-v1alpha1-bgppeer,mutating=false,failurePolicy=fail,groups=metallb.io,resources=bgppeers,versions=v1alpha1,name=bgppeervalidationwebhook.metallb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &BGPPeer{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (bgpPeer *BGPPeer) ValidateCreate() error {
	bgppeerlog.Info("validate create", "name", bgpPeer.Name)
	existingBGPPeersList, err := getExistingBGPPeers()
	if err != nil {
		return err
	}
	return bgpPeer.validateBGPPeer(existingBGPPeersList)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (bgpPeer *BGPPeer) ValidateUpdate(old runtime.Object) error {
	bgppeerlog.Info("validate update", "name", bgpPeer.Name)
	existingBGPPeersList, err := getExistingBGPPeers()
	if err != nil {
		return err
	}
	return bgpPeer.validateBGPPeer(existingBGPPeersList)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (bgpPeer *BGPPeer) ValidateDelete() error {
	bgppeerlog.Info("validate delete", "name", bgpPeer.Name)

	return nil
}

func (bgpPeer *BGPPeer) validateBGPPeer(existingBGPPeersList *BGPPeerList) error {
	var allErrs field.ErrorList

	if err := bgpPeer.validateBGPPeersRouterID(existingBGPPeersList); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := bgpPeer.validateBGPPeerConfig(existingBGPPeersList); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}

	err := apierrors.NewInvalid(
		schema.GroupKind{Group: "metallb.io", Kind: "BGPPeer"},
		bgpPeer.Name, allErrs)
	return err
}

func (bgpPeer *BGPPeer) validateBGPPeersRouterID(existingBGPPeersList *BGPPeerList) *field.Error {
	routerID := bgpPeer.Spec.RouterID

	if len(routerID) == 0 {
		return nil
	}
	if net.ParseIP(routerID) == nil {
		return field.Invalid(field.NewPath("spec").Child("RouterID"), routerID,
			fmt.Sprintf("Invalid RouterID %s", routerID))
	}
	return nil
}

func (bgpPeer *BGPPeer) validateBGPPeerConfig(existingBGPPeersList *BGPPeerList) *field.Error {
	remoteASN := bgpPeer.Spec.ASN
	address := bgpPeer.Spec.Address

	if net.ParseIP(address) == nil {
		return field.Invalid(field.NewPath("spec").Child("Address"), address,
			fmt.Sprintf("Invalid BGPPeer address %s", address))
	}

	for _, BGPPeer := range existingBGPPeersList.Items {
		if remoteASN == BGPPeer.Spec.ASN && address == BGPPeer.Spec.Address {
			return field.Invalid(field.NewPath("spec").Child("Address"), address,
				fmt.Sprintf("Duplicate BGPPeer %s ASN %d in the same BGP instance",
					address, remoteASN))
		}
	}
	return nil
}

func getExistingBGPPeers() (*BGPPeerList, error) {
	existingBGPPeerslList := &BGPPeerList{}
	err := c.List(context.Background(), existingBGPPeerslList)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get existing BGPPeer objects")
	}
	return existingBGPPeerslList, nil
}
