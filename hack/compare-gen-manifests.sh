#!/bin/bash

. $(dirname "$0")/common.sh

METALLB_MANIFESTS_FILE="metallb-native.yaml"

mv config/metallb_rbac/${METALLB_MANIFESTS_FILE} _cache/${METALLB_MANIFESTS_FILE}.rbac

hack/generate-metallb-manifests.sh

diff config/metallb_rbac/${METALLB_MANIFESTS_FILE} _cache/${METALLB_MANIFESTS_FILE}.rbac -q || { echo "Current MetalLB RBAC manifests differ from the RBAC manifests in the MetalLB repo"; exit 1; }
