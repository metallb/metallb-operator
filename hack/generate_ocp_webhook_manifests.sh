#!/bin/bash
. $(dirname "$0")/common.sh

METALLB_PATH=_cache/metallb
FRR_WITH_MANIFESTS_PATH=_cache/metallb-ocp-with-manifests.yaml
MANIFESTS_DIR="bindata/deployment/openshift"
MANIFESTS_FILE="metallb-openshift.yaml"

curl -L https://github.com/metallb/metallb/tarball/"$METALLB_COMMIT_ID" | tar zx -C _cache
rm -rf "$METALLB_PATH"
mv _cache/metallb-metallb-* "$METALLB_PATH"
kubectl kustomize hack/ocp-kustomize-overlay > $FRR_WITH_MANIFESTS_PATH

generate_metallb_frr_manifest ${FRR_WITH_MANIFESTS_PATH} ${MANIFESTS_DIR} ${MANIFESTS_FILE}

rm -rf "$METALLB_PATH"
