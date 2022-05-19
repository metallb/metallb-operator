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

export METALLB_COMMIT_ID="f3c924088a8aea91feec456576b89cba74283eb7"
export METALLB_PATH=_cache/metallb

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
    yq e --inplace '. | (select(.kind == "DaemonSet" and .metadata.name == "speaker") | .spec.template.spec.containers[] | select(.name == "speaker").args)|= . + ["--ml-bindport={{.MLBindPort}}"]' ${manifest_dir}/${manifest_name}
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

    # kube-rbac-proxy modifications
    yq e --inplace ". | select(.kind == \"DaemonSet\" and .metadata.name == \"speaker\").spec.template.spec.volumes += {\"name\": \"{{ if .DeployKubeRbacProxies }}\"}" ${manifest_dir}/${manifest_name}
    yq e --inplace '. | select(.kind == "DaemonSet" and .metadata.name == "speaker").spec.template.spec.volumes += {"name": "speaker-certs", "secret": {"secretName": "speaker-certs-secret"}}' ${manifest_dir}/${manifest_name}
    yq e --inplace ". | select(.kind == \"DaemonSet\" and .metadata.name == \"speaker\").spec.template.spec.volumes += {\"name\": \"{{ end }}\"}" ${manifest_dir}/${manifest_name}

    yq e --inplace ". | select(.kind == \"Deployment\" and .metadata.name == \"controller\").spec.template.spec.volumes += [{\"name\": \"{{ if .DeployKubeRbacProxies }}\"}]" ${manifest_dir}/${manifest_name}
    yq e --inplace '. | select(.kind == "Deployment" and .metadata.name == "controller").spec.template.spec.volumes += {"name": "controller-certs", "secret": {"secretName": "controller-certs-secret"}}' ${manifest_dir}/${manifest_name}
    yq e --inplace ". | select(.kind == \"Deployment\" and .metadata.name == \"controller\").spec.template.spec.volumes += {\"name\": \"{{ end }}\"}" ${manifest_dir}/${manifest_name}

    frr_kube_rbac=`cat $(dirname "$0")/kube-rbac-frr.json | tr -d " \t\n\r"`
    speaker_kube_rbac=`cat $(dirname "$0")/kube-rbac-speaker.json | tr -d " \t\n\r"`
    controller_kube_rbac=`cat $(dirname "$0")/kube-rbac-controller.json | tr -d " \t\n\r"`
    yq e --inplace ". | select(.kind == \"DaemonSet\" and .metadata.name == \"speaker\").spec.template.spec.containers += {\"name\": \"{{ if .DeployKubeRbacProxies }}\"}" ${manifest_dir}/${manifest_name}
    yq e --inplace ". | select(.kind == \"DaemonSet\" and .metadata.name == \"speaker\").spec.template.spec.containers += ${frr_kube_rbac}" ${manifest_dir}/${manifest_name}
    yq e --inplace ". | select(.kind == \"DaemonSet\" and .metadata.name == \"speaker\").spec.template.spec.containers += ${speaker_kube_rbac}" ${manifest_dir}/${manifest_name}
    yq e --inplace ". | select(.kind == \"DaemonSet\" and .metadata.name == \"speaker\").spec.template.spec.containers += {\"name\": \"{{ end }}\"}" ${manifest_dir}/${manifest_name}

    yq e --inplace ". | select(.kind == \"Deployment\" and .metadata.name == \"controller\").spec.template.spec.containers += {\"name\": \"{{ if .DeployKubeRbacProxies }}\"}" ${manifest_dir}/${manifest_name}
    yq e --inplace ". | select(.kind == \"Deployment\" and .metadata.name == \"controller\").spec.template.spec.containers += ${controller_kube_rbac}" ${manifest_dir}/${manifest_name}
    yq e --inplace ". | select(.kind == \"Deployment\" and .metadata.name == \"controller\").spec.template.spec.containers += {\"name\": \"{{ end }}\"}" ${manifest_dir}/${manifest_name}

    # The next part is a bit ugly because we add the sc file content as the securityContext field.
    # The problem with it is that the content is added as a string and not as yaml fields, so we need to use sed to remove yaml's "|-"" mark for them to count as fields.
    # Furthermore, the sed has to be last since it breaks the yaml's syntax by adding the conditionals between
    yq e --inplace '. | select(.kind == "Deployment" and .metadata.name == "controller" and .spec.template.spec.containers[0].name == "controller").spec.template.spec.securityContext|="'"$(< ${METALLB_SC_FILE})"'"' ${manifest_dir}/${manifest_name}
    # Last because they break yaml syntax
    sed -i 's/securityContext\: |-/securityContext\:/g' ${manifest_dir}/${manifest_name}
    sed -i "s/- name: '{{ if .DeployKubeRbacProxies }}'/{{ if .DeployKubeRbacProxies }}/g" ${manifest_dir}/${manifest_name}
    sed -i "s/- name: '{{ end }}'/{{ end }}/g" ${manifest_dir}/${manifest_name}

    sed -i 's/7472/{{.MetricsPort}}/' ${manifest_dir}/${manifest_name}
    sed -i 's/7473/{{.FRRMetricsPort}}/' ${manifest_dir}/${manifest_name}

    sed -i "s/'{{.MetricsPortHttps}}'/{{.MetricsPortHttps}}/" ${manifest_dir}/${manifest_name}
    sed -i "s/'{{.FRRMetricsPortHttps}}'/{{.FRRMetricsPortHttps}}/" ${manifest_dir}/${manifest_name}
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
    yq e --inplace '. | (select(.kind == "DaemonSet" and .metadata.name == "speaker") | .spec.template.spec.containers[] | select(.name == "speaker").args)|= . + ["--ml-bindport={{.MLBindPort}}"]' ${manifest_dir}/${manifest_name}

    yq e --inplace '. | select(.metadata.namespace == "metallb-system").metadata.namespace|="{{.NameSpace}}"' ${manifest_dir}/${manifest_name}
    
    
    # The next part is a bit ugly because we add the sc file content as the securityContext field.
    # The problem with it is that the content is added as a string and not as yaml fields, so we need to use sed to remove yaml's "|-"" mark for them to count as fields.
    # Furthermore, the sed has to be last since it breaks the yaml's syntax by adding the conditionals between
    yq e --inplace '. | select(.kind == "Deployment" and .metadata.name == "controller" and .spec.template.spec.containers[0].name == "controller").spec.template.spec.securityContext|="'"$(< ${METALLB_SC_FILE})"'"' ${manifest_dir}/${manifest_name}
    sed -i 's/securityContext\: |-/securityContext\:/g' ${manifest_dir}/${manifest_name} # Last because it breaks yaml syntax
    sed -i 's/--log-level=info/--log-level={{.LogLevel}}/' ${manifest_dir}/${manifest_name}
    sed -i 's/7472/{{.MetricsPort}}/' ${manifest_dir}/${manifest_name}
    sed -i "s/'{{.MetricsPortHttps}}'/{{.MetricsPortHttps}}/" ${manifest_dir}/${manifest_name}
}

function fetch_metallb() {
    if [[ ! -d "$METALLB_PATH" ]]; then
        curl -L https://github.com/metallb/metallb/tarball/"$METALLB_COMMIT_ID" | tar zx -C _cache
        rm -rf "$METALLB_PATH"
        mv _cache/metallb-metallb-* "$METALLB_PATH"
    fi
}
