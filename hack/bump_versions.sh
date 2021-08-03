#!/bin/bash


metallb_version=$(cat hack/metallb_version.txt)

yq e --inplace '.spec.install.spec.deployments.[0].spec.template.spec.containers[0].env[] |= select (.name=="SPEAKER_IMAGE").value|="quay.io/metallb/speaker:'$metallb_version'"' bundle/manifests/metallb-operator.clusterserviceversion.yaml
yq e --inplace '.spec.install.spec.deployments.[0].spec.template.spec.containers[0].env[] |= select (.name=="CONTROLLER_IMAGE").value|="quay.io/metallb/controller:'$metallb_version'"' bundle/manifests/metallb-operator.clusterserviceversion.yaml
yq e --inplace '.spec.template.spec.containers[0].env[] |= select (.name=="SPEAKER_IMAGE").value|="quay.io/metallb/speaker:'$metallb_version'"' config/manager/env.yaml
yq e --inplace '.spec.template.spec.containers[0].env[] |= select (.name=="CONTROLLER_IMAGE").value|="quay.io/metallb/controller:'$metallb_version'"' config/manager/env.yaml

operator_version=$(cat hack/operator_version.txt)

yq e --inplace '.spec.install.spec.deployments.[0].spec.template.spec.containers[0].image |= "quay.io/metallb/metallb-operator:'$operator_version'"' bundle/manifests/metallb-operator.clusterserviceversion.yaml
yq e --inplace '.images[] |= select (.name == "controller") |= .newTag="'$operator_version'"' config/manager/kustomization.yaml
yq e --inplace '. |= select (.kind == "CatalogSource") |= .spec.image="quay.io/metallb/metallb-operator-bundle-index:'$operator_version'"' config/olm-install/install-resources.yaml
