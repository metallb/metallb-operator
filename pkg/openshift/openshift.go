package openshift

import (
	"context"

	"github.com/Masterminds/semver/v3"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	openshiftapiv1 "github.com/openshift/api/operator/v1"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SupportsFRRK8s(ctx context.Context, cli client.Client) (bool, error) {
	cno := &openshiftconfigv1.ClusterOperator{}
	err := cli.Get(ctx, types.NamespacedName{Name: "network"}, cno)
	if err != nil {
		return false, errors.Wrapf(err, "get openshift network operator failed")
	}
	supports, err := cnoSupportsFRRK8s(cno)
	if err != nil {
		return false, err
	}
	return supports, nil
}

func cnoSupportsFRRK8s(cno *openshiftconfigv1.ClusterOperator) (bool, error) {
	for _, v := range cno.Status.Versions {
		if v.Name == "operator" {
			v, err := semver.NewVersion(v.Version)
			if err != nil {
				return false, errors.Wrapf(err, "failed to parse semver for network operator")
			}
			validVersion, _ := semver.NewVersion("4.17.0-0")
			valid := !v.LessThan(validVersion)
			return valid, nil
		}
	}
	return false, errors.New("failed to find \"operator\" in network operator versions")
}

func DeployFRRK8s(ctx context.Context, cli client.Client) error {
	network := &openshiftapiv1.Network{}
	err := cli.Get(ctx, types.NamespacedName{Name: "cluster"}, network)
	if err != nil {
		return errors.Wrapf(err, "get openshift network failed")
	}
	if network.Spec.DisableNetworkDiagnostics { // TODO replace with the right field
		return nil
	}
	network.Spec.DisableNetworkDiagnostics = true
	err = cli.Update(ctx, network)
	if err != nil {
		return err
	}
	return nil
}
