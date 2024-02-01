#!/bin/bash
. $(dirname "$0")/common.sh

METALLB_MANIFESTS_FILE="metallb-frr-k8s.yaml"
METALLB_MANIFESTS_URL="https://raw.githubusercontent.com/metallb/metallb/${METALLB_COMMIT_ID}/config/manifests/${METALLB_MANIFESTS_FILE}"

HELM_MANIFESTS_DIR="bindata/deployment/helm"
METALLB_HELM_DIR=${HELM_MANIFESTS_DIR}/metallb
FRRK8S_HELM_DIR=${HELM_MANIFESTS_DIR}/frr-k8s

if ! command -v yq &> /dev/null
then
    echo "yq binary not found, installing... "
    go install -mod='' github.com/mikefarah/yq/v4@v4.13.3
fi


curl ${METALLB_MANIFESTS_URL} -o _cache/${METALLB_MANIFESTS_FILE}
# Generate metallb-frr rbac manifests
yq e '. | select(.kind == "Role" or .kind == "ClusterRole" or .kind == "RoleBinding" or .kind == "ClusterRoleBinding" or .kind == "ServiceAccount")' _cache/${METALLB_MANIFESTS_FILE} > config/metallb_rbac/metallb.yaml

fetch_metallb

FRRK8S_VERSION=v$(yq e '.dependencies[] | select(.name == "frr-k8s") | .version'  ${METALLB_PATH}/charts/metallb/Chart.yaml)

fetch_frrk8s $FRRK8S_VERSION
find "$FRRK8S_PATH"/charts/frr-k8s -type f -exec sed -i -e 's/{{ template "frrk8s.fullname" . }}-//g' {} \;

# we want to preserve the metallb crd
ls -d "$METALLB_PATH"/config/crd/bases/* | xargs sed -i '/^---$/d'
ls -d "$METALLB_PATH"/config/crd/bases/* | xargs sed -i '/^$/d'
ls -d "$FRRK8S_PATH"/config/crd/bases/* | xargs sed -i '/^---$/d'
ls -d "$FRRK8S_PATH"/config/crd/bases/* | xargs sed -i '/^$/d'
ls -d config/crd/bases/* | grep -v metallb.io_metallbs | xargs rm
cp -r "$METALLB_PATH"/config/crd/bases config/crd
cp -r "$METALLB_PATH"/config/crd/patches/crd-conversion-patch-bgppeers.yaml config/crd/patches
cp -r "$METALLB_PATH"/config/webhook config/webhook
cp -r "$FRRK8S_PATH"/config/crd/bases config/crd

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

mkdir -p ${METALLB_HELM_DIR}
cp -r "$METALLB_PATH"/charts/metallb/* ${METALLB_HELM_DIR}
rm -rf "$METALLB_PATH"

# generate frr-k8s chart
rm -rf "$FRRK8S_PATH"/charts/frr-k8s/charts
rm -f "$FRRK8S_PATH"/charts/frr-k8s/templates/rbac.yaml
rm -f "$FRRK8S_PATH"/charts/frr-k8s/templates/service-accounts.yaml

yq e --inplace 'del(."dependencies")' "$FRRK8S_PATH"/charts/frr-k8s/Chart.yaml

sed -i -e 's/app.kubernetes.io\///g' "$FRRK8S_PATH"/charts/frr-k8s/templates/controller.yaml
sed -i -e 's/app.kubernetes.io\///g' "$FRRK8S_PATH"/charts/frr-k8s/templates/webhooks.yaml
sed -i -e 's/name: webhook-server/name: frr-k8s-webhook-server/g' "$FRRK8S_PATH"/charts/frr-k8s/templates/webhooks.yaml
sed -i -e 's/app.kubernetes.io\///g' "$FRRK8S_PATH"/charts/frr-k8s/templates/service-monitor.yaml

sed -i '/app.kubernetes.io\/instance: {{ .Release.Name }}/d' "$FRRK8S_PATH"/charts/frr-k8s/templates/_helpers.tpl
sed -i -e 's/app.kubernetes.io\/name/app/g' "$FRRK8S_PATH"/charts/frr-k8s/templates/_helpers.tpl

mkdir -p ${FRRK8S_HELM_DIR}
cp -r "$FRRK8S_PATH"/charts/frr-k8s/* ${FRRK8S_HELM_DIR}
rm -rf "$FRRK8S_PATH"
