package controllers

import (
	"context"
	"sort"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metallbv1alpha1 "github.com/metallb/metallb-operator/api/v1alpha1"
	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	"github.com/metallb/metallb-operator/pkg/render"
	corev1 "k8s.io/api/core/v1"
)

const ConfigMapName = "config"
const ConfigDataField = "config"

func reconcileConfigMap(ctx context.Context, c client.Client, log logr.Logger, namespace string) error {
	config, err := operatorConfig(ctx, c)
	if err != nil {
		return errors.Wrap(err, "failed to collect configmap data")
	}
	config.NameSpace = namespace
	config.ConfigMapName = ConfigMapName
	rendered, err := render.OperatorConfigToMetalLB(config)
	if err != nil {
		return err
	}

	existing := &corev1.ConfigMap{}
	err = c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: ConfigMapName}, existing)
	if k8serrors.IsNotFound(err) {
		return c.Create(ctx, rendered)
	}

	if existing.Data[ConfigDataField] == rendered.Data[ConfigDataField] {
		log.Info("not updating configmap because of no changes")
		return nil
	}
	err = c.Update(ctx, rendered)
	if err != nil {
		return errors.Wrap(err, "failed to update configmap")
	}
	return nil
}

func operatorConfig(ctx context.Context, c client.Client) (*render.OperatorConfig, error) {
	addressPools := &metallbv1alpha1.AddressPoolList{}
	err := c.List(ctx, addressPools, &client.ListOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, errors.Wrap(err, "failed to fetch address pools")
	}

	bgpPeers := &metallbv1alpha1.BGPPeerList{}
	err = c.List(ctx, bgpPeers, &client.ListOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, errors.Wrap(err, "failed to fetch bgp peers")
	}

	bfdProfiles := &metallbv1beta1.BFDProfileList{}
	err = c.List(ctx, bfdProfiles, &client.ListOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, errors.Wrap(err, "failed to fetch bfd profiles")
	}
	res := &render.OperatorConfig{}
	res.Pools = addressPools.DeepCopy().Items
	res.Peers = bgpPeers.DeepCopy().Items
	res.BFDProfiles = bfdProfiles.DeepCopy().Items

	// sorting to make the result stable in case the api server returns the list in
	// a different order.
	sort.Slice(res.Pools, func(i, j int) bool { return res.Pools[i].Name < res.Pools[j].Name })
	sort.Slice(res.Peers, func(i, j int) bool { return res.Peers[i].Name < res.Peers[j].Name })
	sort.Slice(res.BFDProfiles, func(i, j int) bool { return res.BFDProfiles[i].Name < res.BFDProfiles[j].Name })

	res.DataField = ConfigDataField
	return res, nil
}
