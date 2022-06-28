#!/bin/bash

HELM_DIRECTORY="helm"
VERSION="main"
HELM_MANIFESTS_DIR="bindata/deployment/helm"

wget https://github.com/metallb/metallb/archive/refs/heads/${VERSION}.zip -P _cache/${HELM_DIRECTORY}
unzip _cache/${HELM_DIRECTORY}/${VERSION}.zip -d _cache/${HELM_DIRECTORY}

mkdir -p ${HELM_MANIFESTS_DIR}
cp -r _cache/${HELM_DIRECTORY}/metallb-${VERSION}/charts/metallb/* ${HELM_MANIFESTS_DIR}

rm -rf _cache/${HELM_DIRECTORY}
