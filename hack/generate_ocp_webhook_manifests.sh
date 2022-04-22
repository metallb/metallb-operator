#!/bin/bash
. $(dirname "$0")/common.sh

FRR_WITH_MANIFESTS_PATH=_cache/metallb-ocp-with-manifests.yaml
MANIFESTS_DIR="bindata/deployment/openshift"
MANIFESTS_FILE="metallb-openshift.yaml"

fetch_metallb
kubectl kustomize hack/ocp-kustomize-overlay > $FRR_WITH_MANIFESTS_PATH

generate_metallb_frr_manifest ${FRR_WITH_MANIFESTS_PATH} ${MANIFESTS_DIR} ${MANIFESTS_FILE}

rm -rf "$METALLB_PATH"
