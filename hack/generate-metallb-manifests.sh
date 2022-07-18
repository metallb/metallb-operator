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

HELM_MANIFESTS_DIR="bindata/deployment/helm"

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
cp -r "$METALLB_PATH"/config/crd/crd-conversion-patch.yaml config/crd
cp -r "$METALLB_PATH"/config/webhook config/webhook

# generate metallb chart
rm -rf "$METALLB_PATH"/charts/metallb/charts
rm -f "$METALLB_PATH"/charts/metallb/templates/rbac.yaml
rm -f "$METALLB_PATH"/charts/metallb/templates/service-accounts.yaml
rm -f "$METALLB_PATH"/charts/metallb/templates/webhooks.yaml

yq e --inplace 'del(."dependencies")' "$METALLB_PATH"/charts/metallb/Chart.yaml

find "$METALLB_PATH"/charts/metallb -type f -exec sed -i -e 's/{{ template "metallb.fullname" . }}-//g' {} \;
sed -i -e 's/app.kubernetes.io\///g' "$METALLB_PATH"/charts/metallb/templates/controller.yaml
sed -i -e 's/metallb-webhook-service/webhook-service/g' "$METALLB_PATH"/charts/metallb/templates/controller.yaml
sed -i -e 's/app.kubernetes.io\/component/component/g' "$METALLB_PATH"/charts/metallb/templates/speaker.yaml
sed -i -e 's/app.kubernetes.io\/name/app/g' "$METALLB_PATH"/charts/metallb/templates/speaker.yaml
sed -i '/app.kubernetes.io\/instance: {{ .Release.Name }}/d' "$METALLB_PATH"/charts/metallb/templates/_helpers.tpl
sed -i -e 's/app.kubernetes.io\/name/app/g' "$METALLB_PATH"/charts/metallb/templates/_helpers.tpl

mkdir -p ${HELM_MANIFESTS_DIR}
cp -r "$METALLB_PATH"/charts/metallb/* ${HELM_MANIFESTS_DIR}
rm -rf "$METALLB_PATH"
