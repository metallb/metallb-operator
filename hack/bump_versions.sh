#!/bin/bash


metallb_version=$(cat hack/metallb_version.txt)

yq e --inplace '.spec.install.spec.deployments.[0].spec.template.spec.containers[0].env[] |= select (.name=="SPEAKER_IMAGE").value|="quay.io/metallb/speaker:'$metallb_version'"' bundle/manifests/metallb-operator.clusterserviceversion.yaml
yq e --inplace '.spec.install.spec.deployments.[0].spec.template.spec.containers[0].env[] |= select (.name=="CONTROLLER_IMAGE").value|="quay.io/metallb/controller:'$metallb_version'"' bundle/manifests/metallb-operator.clusterserviceversion.yaml
yq e --inplace '.spec.template.spec.containers[0].env[] |= select (.name=="SPEAKER_IMAGE").value|="quay.io/metallb/speaker:'$metallb_version'"' config/manager/env.yaml
yq e --inplace '.spec.template.spec.containers[0].env[] |= select (.name=="CONTROLLER_IMAGE").value|="quay.io/metallb/controller:'$metallb_version'"' config/manager/env.yaml

operator_version=$(cat hack/operator_version.txt)
csv_version=$(echo "$operator_version" | sed 's/v//')
if [ $operator_version = "latest" ]; then # operator sdk doesn't like string versions, if we are on main we don't care about the version in the csv
    csv_version="0.0.0" 
fi

yq e --inplace '.spec.install.spec.deployments.[0].spec.template.spec.containers[0].image |= "quay.io/metallb/metallb-operator:'$operator_version'"' bundle/manifests/metallb-operator.clusterserviceversion.yaml
yq e --inplace '.spec.version |= "'$csv_version'"' bundle/manifests/metallb-operator.clusterserviceversion.yaml
yq e --inplace '.images[] |= select (.name == "controller") |= .newTag="'$operator_version'"' config/manager/kustomization.yaml

if [ "$operator_version" != "latest" ]; then
sed -i "s/name: metallb-operator.v0.0.0/name: metallb-operator.$operator_version/" bundle/manifests/metallb-operator.clusterserviceversion.yaml
fi

yq e --inplace '. |= select (.kind == "CatalogSource") |= .spec.image="quay.io/metallb/metallb-operator-bundle-index:'$operator_version'"' config/olm-install/install-resources.yaml

sed -E -i "s/VERSION \?= .*$/VERSION \?= $operator_version/g" Makefile

sed -i "s/quay.io\/metallb\/speaker:main/quay.io\/metallb\/speaker:$metallb_version/g" bin/metallb-operator.yaml
sed -i "s/quay.io\/metallb\/controller:main/quay.io\/metallb\/controller:$metallb_version/g" bin/metallb-operator.yaml
sed -i "s/quay.io\/metallb\/metallb-operator:latest/quay.io\/metallb\/metallb-operator:$operator_version/g" bin/metallb-operator.yaml
