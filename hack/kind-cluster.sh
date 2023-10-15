#!/bin/bash
set -o errexit

KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-kind}"
KIND_VERSION=""
CLUSTER_CONFIG=""
REGISTRY="false"
REGISTRY_PORT="5000"
REGISTRY_NAME="kind-registry"

usage() {
	echo "Usage: $0 --kind-version <version> --cluster-config <file> [--registry true|false]"
}

startRegistry() {
	IS_REGISTRY_RUNNING="$(docker inspect -f '{{.State.Running}}' "${REGISTRY_NAME}" 2>/dev/null || true)"
	if [ "${IS_REGISTRY_RUNNING}" != 'true' ]; then
		docker run --detach \
			--restart=always \
			--name "${REGISTRY_NAME}" \
			--publish "${REGISTRY_PORT}:5000" \
			registry:2
	fi
}

patchRegistryConfig() {
	cat << EOF | yq eval-all 'select(fileIndex == 0) * select(fileIndex == 1)' ${CLUSTER_CONFIG} -
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${REGISTRY_PORT}"]
  endpoint = ["http://${REGISTRY_NAME}:${REGISTRY_PORT}"]
EOF
}

registryConfigMap() {
	cat << EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${REGISTRY_PORT}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
		--kind-version)
			KIND_VERSION="$2"
			shift 2
			;;
		--cluster-config)
			CLUSTER_CONFIG="$2"
			shift 2
			;;
		--registry)
			REGISTRY="$2"
			shift 2
			;;
		*)
			usage
			exit 1
			;;
	esac
done

if [[ -z "${KIND_VERSION}" || -z "${CLUSTER_CONFIG}" ]]; then
	usage
	exit 1
else
	if [ "${REGISTRY}" = "true" ]; then
		startRegistry
		kind delete cluster --name ${KIND_CLUSTER_NAME} || true
		kind create cluster --image kindest/node:${KIND_VERSION} --name ${KIND_CLUSTER_NAME} --config=<(patchRegistryConfig)
		registryConfigMap
		docker network connect ${KIND_CLUSTER_NAME} ${REGISTRY_NAME}
	else
		kind delete cluster --name ${KIND_CLUSTER_NAME} || true
		kind create cluster --image kindest/node:${KIND_VERSION} --name ${KIND_CLUSTER_NAME} --config=${CLUSTER_CONFIG}
	fi
fi
