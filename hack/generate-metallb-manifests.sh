#!/bin/bash
. $(dirname "$0")/common.sh

NATIVE_MANIFESTS_FILE="metallb-native.yaml"
NATIVE_MANIFESTS_URL="https://raw.githubusercontent.com/metallb/metallb/${METALLB_COMMIT_ID}/config/manifests/${NATIVE_MANIFESTS_FILE}"

FRR_MANIFESTS_FILE="metallb-frr.yaml"
FRR_MANIFESTS_URL="https://raw.githubusercontent.com/metallb/metallb/${METALLB_COMMIT_ID}/config/manifests/${FRR_MANIFESTS_FILE}"

HELM_MANIFESTS_DIR="bindata/deployment/helm"

if ! command -v yq &> /dev/null
then
    echo "yq binary not found, installing... "
    go install -mod='' github.com/mikefarah/yq/v4@v4.13.3
fi

curl ${NATIVE_MANIFESTS_URL} -o _cache/${NATIVE_MANIFESTS_FILE}
# Generate metallb-native rbac manifests
yq e '. | select(.kind == "Role" or .kind == "ClusterRole" or .kind == "RoleBinding" or .kind == "ClusterRoleBinding" or .kind == "ServiceAccount")' _cache/${NATIVE_MANIFESTS_FILE} > config/metallb_rbac/${NATIVE_MANIFESTS_FILE}


curl ${FRR_MANIFESTS_URL} -o _cache/${FRR_MANIFESTS_FILE}
# Generate metallb-frr rbac manifests
yq e '. | select(.kind == "Role" or .kind == "ClusterRole" or .kind == "RoleBinding" or .kind == "ClusterRoleBinding" or .kind == "ServiceAccount")' _cache/${FRR_MANIFESTS_FILE} > config/metallb_rbac/${FRR_MANIFESTS_FILE}

fetch_metallb

# we want to preserve the metallb crd
ls -d "$METALLB_PATH"/config/crd/bases/* | xargs sed -i '/^---$/d'
ls -d "$METALLB_PATH"/config/crd/bases/* | xargs sed -i '/^$/d'
ls -d config/crd/bases/* | grep -v metallb.io_metallbs | xargs rm 
cp -r "$METALLB_PATH"/config/crd/bases config/crd
cp -r "$METALLB_PATH"/config/crd/patches/crd-conversion-patch-addresspools.yaml config/crd/patches
cp -r "$METALLB_PATH"/config/crd/patches/crd-conversion-patch-bgppeers.yaml config/crd/patches
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
sed -i -e 's/app.kubernetes.io\/component/component/g' "$METALLB_PATH"/charts/metallb/templates/servicemonitor.yaml
sed -i -e 's/app.kubernetes.io\/name/app/g' "$METALLB_PATH"/charts/metallb/templates/speaker.yaml
sed -i '/app.kubernetes.io\/instance: {{ .Release.Name }}/d' "$METALLB_PATH"/charts/metallb/templates/_helpers.tpl
sed -i -e 's/app.kubernetes.io\/name/app/g' "$METALLB_PATH"/charts/metallb/templates/_helpers.tpl

mkdir -p ${HELM_MANIFESTS_DIR}
cp -r "$METALLB_PATH"/charts/metallb/* ${HELM_MANIFESTS_DIR}
rm -rf "$METALLB_PATH"
