#!/bin/bash
. $(dirname "$0")/common.sh

NATIVE_MANIFESTS_FILE="metallb-native.yaml"
NATIVE_MANIFESTS_URL="https://raw.githubusercontent.com/metallb/metallb/${METALLB_COMMIT_ID}/config/manifests/${NATIVE_MANIFESTS_FILE}"
NATIVE_MANIFESTS_DIR="bindata/deployment/native"

FRR_MANIFESTS_FILE="metallb-frr.yaml"
FRR_MANIFESTS_URL="https://raw.githubusercontent.com/metallb/metallb/${METALLB_COMMIT_ID}/config/manifests/${FRR_MANIFESTS_FILE}"
FRR_MANIFESTS_DIR="bindata/deployment/frr"

PROMETHEUS_OPERATOR_FILE="prometheus-operator.yaml"
PROMETHEUS_OPERATOR_MANIFESTS_URL="https://raw.githubusercontent.com/metallb/metallb/${METALLB_COMMIT_ID}/config/prometheus/${PROMETHEUS_OPERATOR_FILE}"
PROMETHEUS_OPERATOR_MANIFESTS_DIR="bindata/deployment/prometheus-operator"

if ! command -v yq &> /dev/null
then
    echo "yq binary not found, installing... "
    go install -mod='' github.com/mikefarah/yq/v4@v4.13.3
fi

curl ${NATIVE_MANIFESTS_URL} -o _cache/${NATIVE_MANIFESTS_FILE}
generate_metallb_native_manifest _cache/${NATIVE_MANIFESTS_FILE} ${NATIVE_MANIFESTS_DIR} ${NATIVE_MANIFESTS_FILE}

curl ${FRR_MANIFESTS_URL} -o _cache/${FRR_MANIFESTS_FILE}
generate_metallb_frr_manifest _cache/${FRR_MANIFESTS_FILE} ${FRR_MANIFESTS_DIR} ${FRR_MANIFESTS_FILE}

# Update MetalLB's E2E lane to clone the same commit as the manifests.
yq e --inplace ".jobs.main.steps[] |= select(.name==\"Checkout MetalLB\").with.ref=\"${METALLB_COMMIT_ID}\"" .github/workflows/metallb_e2e.yml

# TODO: run this script once FRR is merged

# Prometheus Operator manifests
curl ${PROMETHEUS_OPERATOR_MANIFESTS_URL} -o _cache/${PROMETHEUS_OPERATOR_FILE}
yq e '. | select((.kind == "Role" or .kind == "ClusterRole" or .kind == "RoleBinding" or .kind == "ClusterRoleBinding" or .kind == "ServiceAccount") | not)' _cache/${PROMETHEUS_OPERATOR_FILE} > ${PROMETHEUS_OPERATOR_MANIFESTS_DIR}/${PROMETHEUS_OPERATOR_FILE}
yq e --inplace '. | select(.kind == "PodMonitor").metadata.namespace|="{{.NameSpace}}"' ${PROMETHEUS_OPERATOR_MANIFESTS_DIR}/${PROMETHEUS_OPERATOR_FILE}
yq e --inplace '. | select(.kind == "PodMonitor").spec.namespaceSelector.matchNames|=["{{.NameSpace}}"]' ${PROMETHEUS_OPERATOR_MANIFESTS_DIR}/${PROMETHEUS_OPERATOR_FILE}

fetch_metallb

# we want to preserve the metallb crd
ls -d "$METALLB_PATH"/config/crd/bases/* | xargs sed -i '/^---$/d'
ls -d "$METALLB_PATH"/config/crd/bases/* | xargs sed -i '/^$/d'
ls -d config/crd/bases/* | grep -v metallb.io_metallbs | xargs rm 
cp -r "$METALLB_PATH"/config/crd/bases config/crd
ls -d bundle/manifests/metallb.io_* | grep -v metallb.io_metallbs | xargs rm 
cp -r "$METALLB_PATH"/config/crd/bases/* bundle/manifests/
ls -d bundle/manifests/metallb.io_* 
