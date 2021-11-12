#!/bin/bash

. $(dirname "$0")/common.sh

METALLB_COMMIT_ID="312b03cd3065687f25274486cd3ff5c79d6f6068"
METALLB_MANIFESTS_URL="https://raw.githubusercontent.com/metallb/metallb/${METALLB_COMMIT_ID}/manifests/metallb.yaml"
METALLB_MANIFESTS_DIR="bindata/deployment"
METALLB_MANIFESTS_FILE="metallb.yaml"
METALLB_SC_FILE=$(dirname "$0")/securityContext.yaml

if ! command -v yq &> /dev/null
then
    echo "yq binary not found, installing... "
    go install -mod='' github.com/mikefarah/yq/v4@v4.13.3
fi

curl ${METALLB_MANIFESTS_URL} -o _cache/${METALLB_MANIFESTS_FILE}

# Generate metallb rbac manifests
yq e '. | select(.kind == "Role" or .kind == "ClusterRole" or .kind == "RoleBinding" or .kind == "ClusterRoleBinding" or .kind == "ServiceAccount")' _cache/${METALLB_MANIFESTS_FILE} > config/metallb_rbac/${METALLB_MANIFESTS_FILE}

# Generate metallb deployment manifests
yq e '. | select((.kind == "Role" or .kind == "ClusterRole" or .kind == "RoleBinding" or .kind == "ClusterRoleBinding" or .kind == "ServiceAccount") | not)' _cache/${METALLB_MANIFESTS_FILE} > ${METALLB_MANIFESTS_DIR}/${METALLB_MANIFESTS_FILE}

# Editing manifests to include templated variables
yq e --inplace '. | select(.kind == "Deployment" and .metadata.name == "controller" and .spec.template.spec.containers[0].name == "controller").spec.template.spec.containers[0].image|="{{.ControllerImage}}"' ${METALLB_MANIFESTS_DIR}/${METALLB_MANIFESTS_FILE}
yq e --inplace '. | select(.kind == "Deployment" and .metadata.name == "controller" and .spec.template.spec.containers[0].name == "controller").spec.template.spec.containers[0].command|= ["/controller"]' ${METALLB_MANIFESTS_DIR}/${METALLB_MANIFESTS_FILE}
yq e --inplace '. | select(.kind == "DaemonSet" and .metadata.name == "speaker" and .spec.template.spec.containers[0].name == "speaker").spec.template.spec.containers[0].image|="{{.SpeakerImage}}"' ${METALLB_MANIFESTS_DIR}/${METALLB_MANIFESTS_FILE}
yq e --inplace '. | select(.kind == "DaemonSet" and .metadata.name == "speaker" and .spec.template.spec.containers[0].name == "speaker").spec.template.spec.containers[0].command|= ["/speaker"]' ${METALLB_MANIFESTS_DIR}/${METALLB_MANIFESTS_FILE}
yq e --inplace '. | select(.metadata.namespace == "metallb-system").metadata.namespace|="{{.NameSpace}}"' ${METALLB_MANIFESTS_DIR}/${METALLB_MANIFESTS_FILE}
# The next part is a bit ugly because we add the sc file content as the securityContext field.
# The problem with it is that the content is added as a string and not as yaml fields, so we need to use sed to remove yaml's "|-"" mark for them to count as fields.
# Furthermore, the sed has to be last since it breaks the yaml's syntax by adding the conditionals between
yq e --inplace '. | select(.kind == "Deployment" and .metadata.name == "controller" and .spec.template.spec.containers[0].name == "controller").spec.template.spec.securityContext|="'"$(< ${METALLB_SC_FILE})"'"' ${METALLB_MANIFESTS_DIR}/${METALLB_MANIFESTS_FILE}
sed -i 's/securityContext\: |-/securityContext\:/g' ${METALLB_MANIFESTS_DIR}/${METALLB_MANIFESTS_FILE} # Last because it breaks yaml syntax
