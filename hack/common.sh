#!/bin/bash

set -e

pushd .
cd "$(dirname "$0")/.."

function finish {
    popd
}
trap finish EXIT

GOPATH="${GOPATH:-~/go}"
export GOFLAGS="${GOFLAGS:-"-mod=vendor"}"

export PATH=$PATH:$GOPATH/bin

mkdir -p _cache

export METALLB_COMMIT_ID="35e41dea662b43e91b5c2a7338b92d27eb059a4d"

export METALLB_SC_FILE=$(dirname "$0")/securityContext.yaml

function generate_metallb_frr_manifest() {
    source_file=$1
    manifest_dir=$2
    manifest_name=$3

    # Generate metallb-frr rbac manifests
    yq e '. | select(.kind == "Role" or .kind == "ClusterRole" or .kind == "RoleBinding" or .kind == "ClusterRoleBinding" or .kind == "ServiceAccount")' ${source_file} > config/metallb_rbac/${manifest_name}

    # Generate metallb-frr deployment manifests
    yq e '. | select((.kind == "Role" or .kind == "ClusterRole" or .kind == "RoleBinding" or .kind == "ClusterRoleBinding" or .kind == "ServiceAccount" or .kind == "CustomResourceDefinition"  or .kind == "Namespace") | not)' ${source_file} > ${manifest_dir}/${manifest_name}

    # Editing metallb-frr manifests to include templated variables
    yq e --inplace '. | select(.kind == "Deployment" and .metadata.name == "controller" and .spec.template.spec.containers[0].name == "controller").spec.template.spec.containers[0].image|="{{.ControllerImage}}"' ${manifest_dir}/${manifest_name}
    yq e --inplace '. | select(.kind == "Deployment" and .metadata.name == "controller" and .spec.template.spec.containers[0].name == "controller").spec.template.spec.containers[0].command|= ["/controller"]' ${manifest_dir}/${manifest_name}
    yq e --inplace '. | (select(.kind == "DaemonSet" and .metadata.name == "speaker") | .spec.template.spec.containers[] | select(.name == "speaker").image)|="{{.SpeakerImage}}"' ${manifest_dir}/${manifest_name}
    yq e --inplace '. | (select(.kind == "DaemonSet" and .metadata.name == "speaker") | .spec.template.spec.containers[] | select(.name == "speaker").command)|= ["/speaker"]' ${manifest_dir}/${manifest_name}
    yq e --inplace '. | (select(.kind == "DaemonSet" and .metadata.name == "speaker") | .spec.template.spec.containers[] | select(.name == "frr").image)|="{{.FRRImage}}"' ${manifest_dir}/${manifest_name}
    yq e --inplace '. | (select(.kind == "DaemonSet" and .metadata.name == "speaker") | .spec.template.spec.containers[] | select(.name == "reloader").image)|="{{.FRRImage}}"' ${manifest_dir}/${manifest_name}
    yq e --inplace '. | (select(.kind == "DaemonSet" and .metadata.name == "speaker") | .spec.template.spec.containers[] | select(.name == "frr-metrics").image)|="{{.FRRImage}}"' ${manifest_dir}/${manifest_name}
    yq e --inplace '. | (select(.kind == "DaemonSet" and .metadata.name == "speaker") | .spec.template.spec.initContainers[] | select(.name == "cp-frr-files").image)|="{{.FRRImage}}"' ${manifest_dir}/${manifest_name}
    yq e --inplace '. | (select(.kind == "DaemonSet" and .metadata.name == "speaker") | .spec.template.spec.initContainers[] | select(.name == "cp-reloader").image)|="{{.SpeakerImage}}"' ${manifest_dir}/${manifest_name}
    yq e --inplace '. | (select(.kind == "DaemonSet" and .metadata.name == "speaker") | .spec.template.spec.initContainers[] | select(.name == "cp-metrics").image)|="{{.SpeakerImage}}"' ${manifest_dir}/${manifest_name}
    yq e --inplace '. | select(.kind == "Deployment" and .metadata.name == "controller" and .spec.template.spec.containers[0].name == "controller" and .spec.template.spec.securityContext.runAsUser == "65534").spec.template.spec.securityContext|="'"$(< ${METALLB_SC_FILE})"'"' ${manifest_dir}/${manifest_name}
    yq e --inplace '. | select(.metadata.namespace == "metallb-system").metadata.namespace|="{{.NameSpace}}"' ${manifest_dir}/${manifest_name}
    sed -i 's/--log-level=info/--log-level={{.LogLevel}}/' ${manifest_dir}/${manifest_name}
    sed -i '/- name: FRR_LOGGING_LEVEL/ s//# &/' ${manifest_dir}/${manifest_name}
    sed -i '/  value: informational/ s//# &/' ${manifest_dir}/${manifest_name}

    # The next part is a bit ugly because we add the sc file content as the securityContext field.
    # The problem with it is that the content is added as a string and not as yaml fields, so we need to use sed to remove yaml's "|-"" mark for them to count as fields.
    # Furthermore, the sed has to be last since it breaks the yaml's syntax by adding the conditionals between
    yq e --inplace '. | select(.kind == "Deployment" and .metadata.name == "controller" and .spec.template.spec.containers[0].name == "controller").spec.template.spec.securityContext|="'"$(< ${METALLB_SC_FILE})"'"' ${manifest_dir}/${manifest_name}
    # Last because they break yaml syntax
    sed -i 's/securityContext\: |-/securityContext\:/g' ${manifest_dir}/${manifest_name}
    sed -i "s/- name: '{{ if .DeployKubeRbacProxies }}'/{{ if .DeployKubeRbacProxies }}/g" ${manifest_dir}/${manifest_name}
    sed -i "s/- name: '{{ end }}'/{{ end }}/g" ${manifest_dir}/${manifest_name}
}


function generate_metallb_native_manifest() {
    source_file=$1
    manifest_dir=$2
    manifest_name=$3

    # Generate metallb rbac manifests
    yq e '. | select(.kind == "Role" or .kind == "ClusterRole" or .kind == "RoleBinding" or .kind == "ClusterRoleBinding" or .kind == "ServiceAccount")' ${source_file} > config/metallb_rbac/${manifest_name}

    # Generate metallb deployment manifests
    yq e '. | select((.kind == "Role" or .kind == "ClusterRole" or .kind == "RoleBinding" or .kind == "ClusterRoleBinding" or .kind == "ServiceAccount" or .kind == "CustomResourceDefinition" or .kind == "Namespace") | not)' ${source_file} > ${manifest_dir}/${manifest_name}

    # Editing metallb manifests to include templated variables
    yq e --inplace '. | select(.kind == "Deployment" and .metadata.name == "controller" and .spec.template.spec.containers[0].name == "controller").spec.template.spec.containers[0].image|="{{.ControllerImage}}"' ${manifest_dir}/${manifest_name}
    yq e --inplace '. | select(.kind == "Deployment" and .metadata.name == "controller" and .spec.template.spec.containers[0].name == "controller").spec.template.spec.containers[0].command|= ["/controller"]' ${manifest_dir}/${manifest_name}
    yq e --inplace '. | select(.kind == "DaemonSet" and .metadata.name == "speaker" and .spec.template.spec.containers[0].name == "speaker").spec.template.spec.containers[0].image|="{{.SpeakerImage}}"' ${manifest_dir}/${manifest_name}
    yq e --inplace '. | select(.kind == "DaemonSet" and .metadata.name == "speaker" and .spec.template.spec.containers[0].name == "speaker").spec.template.spec.containers[0].command|= ["/speaker"]' ${manifest_dir}/${manifest_name}
    yq e --inplace '. | select(.metadata.namespace == "metallb-system").metadata.namespace|="{{.NameSpace}}"' ${manifest_dir}/${manifest_name}
    # The next part is a bit ugly because we add the sc file content as the securityContext field.
    # The problem with it is that the content is added as a string and not as yaml fields, so we need to use sed to remove yaml's "|-"" mark for them to count as fields.
    # Furthermore, the sed has to be last since it breaks the yaml's syntax by adding the conditionals between
    yq e --inplace '. | select(.kind == "Deployment" and .metadata.name == "controller" and .spec.template.spec.containers[0].name == "controller").spec.template.spec.securityContext|="'"$(< ${METALLB_SC_FILE})"'"' ${manifest_dir}/${manifest_name}
    sed -i 's/securityContext\: |-/securityContext\:/g' ${manifest_dir}/${manifest_name} # Last because it breaks yaml syntax
    sed -i 's/--log-level=info/--log-level={{.LogLevel}}/' ${manifest_dir}/${manifest_name}
}
