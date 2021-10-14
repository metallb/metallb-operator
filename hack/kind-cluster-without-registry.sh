#!/usr/bin/env bash
set -o errexit

# desired cluster name; default is "kind"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-kind}"

cat <<EOF | kind create cluster --image kindest/node:v1.21.1 --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
        kubeletExtraArgs:
        container-log-max-size: "100Mi"
  - |
    kind: ClusterConfiguration
    apiServer:
        extraArgs:
            v: "5"
    controllerManager:
        extraArgs:
            v: "5"
    scheduler:
        extraArgs:
            v: "5"
EOF
