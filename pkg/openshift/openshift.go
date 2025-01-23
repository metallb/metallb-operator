package openshift

import (
	"context"
	"slices"

	"github.com/Masterminds/semver/v3"
	"github.com/metallb/metallb-operator/pkg/params"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	openshiftapiv1 "github.com/openshift/api/operator/v1"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SupportsFRRK8s(ctx context.Context, cli client.Client, envConfig params.EnvConfig) (bool, error) {
	cno := &openshiftconfigv1.ClusterOperator{}
	err := cli.Get(ctx, types.NamespacedName{Name: "network"}, cno)
	if err != nil {
		return false, errors.Wrapf(err, "get openshift network operator failed")
	}
	supports, err := cnoSupportsFRRK8s(cno, envConfig)
	if err != nil {
		return false, err
	}
	return supports, nil
}

func cnoSupportsFRRK8s(cno *openshiftconfigv1.ClusterOperator, envConfig params.EnvConfig) (bool, error) {
	for _, v := range cno.Status.Versions {
		if v.Name == "operator" {
			v, err := semver.NewVersion(v.Version)
			if err != nil {
				return false, errors.Wrapf(err, "failed to parse semver for network operator")
			}
			validVersion, _ := semver.NewVersion(envConfig.CNOMinFRRK8sVersion)
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
	if network.Spec.AdditionalRoutingCapabilities == nil {
		network.Spec.AdditionalRoutingCapabilities = &openshiftapiv1.AdditionalRoutingCapabilities{}
	}
	if slices.Contains(network.Spec.AdditionalRoutingCapabilities.Providers, openshiftapiv1.RoutingCapabilitiesProviderFRR) {
		return nil
	}
	network.Spec.AdditionalRoutingCapabilities.Providers = append(network.Spec.AdditionalRoutingCapabilities.Providers, openshiftapiv1.RoutingCapabilitiesProviderFRR)
	err = cli.Update(ctx, network)
	if err != nil {
		return err
	}
	return nil
}
