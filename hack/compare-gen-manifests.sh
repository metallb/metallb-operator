#!/bin/bash

. $(dirname "$0")/common.sh

METALLB_MANIFESTS_DIR="bindata/deployment"
METALLB_MANIFESTS_FILE="metallb.yaml"

mv ${METALLB_MANIFESTS_DIR}/${METALLB_MANIFESTS_FILE} _cache/${METALLB_MANIFESTS_FILE}.manifests
mv config/metallb_rbac/${METALLB_MANIFESTS_FILE} _cache/${METALLB_MANIFESTS_FILE}.rbac

hack/generate-metallb-manifests.sh

diff ${METALLB_MANIFESTS_DIR}/${METALLB_MANIFESTS_FILE} _cache/${METALLB_MANIFESTS_FILE}.manifests -q || { echo "Current MetalLB manifests differ from the manifests in the MetalLB repo"; exit 1; }
diff config/metallb_rbac/${METALLB_MANIFESTS_FILE} _cache/${METALLB_MANIFESTS_FILE}.rbac -q || { echo "Current MetalLB RBAC manifests differ from the RBAC manifests in the MetalLB repo"; exit 1; }
