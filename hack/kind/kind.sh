#!/usr/bin/env bash
set -o errexit
set -x

# desired cluster name; default is "kind"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-kind}"
KIND_BIN="${KIND_BIN:-kind}"

clusters=$("${KIND_BIN}" get clusters)
for cluster in $clusters; do
  if [[ $cluster == "$KIND_CLUSTER_NAME" ]]; then
    echo "Cluster ${KIND_CLUSTER_NAME} already exists"
    exit 0
  fi
done

CONFIG_NAME="hack/kind/config_vanilla.yaml"

if [[ ! -z $KIND_WITH_REGISTRY ]]; then
  CONFIG_NAME="hack/kind/config_with_registry.yaml"

# create registry container unless it already exists
running="$(docker inspect -f '{{.State.Running}}' "kind-registry" 2>/dev/null || true)"
if [ "${running}" != 'true' ]; then
  docker run \
    -d --restart=always -p "5000:5000" --name "kind-registry" \
    registry:2
fi

fi

# create a cluster with the local registry enabled in containerd
"${KIND_BIN}" create cluster --name "${KIND_CLUSTER_NAME}" --config=${CONFIG_NAME}

if [[ ! -z $KIND_WITH_REGISTRY ]]; then

# connect the registry to the cluster network
docker network connect "kind" "kind-registry" || true

# Document the local registry
# https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry
kubectl apply -f hack/kind/registry_configmap.yaml

fi

if docker network ls | grep -q network2; then
  docker network rm network2
fi
# Create a second network interface, useful for metallb's e2e tests
docker network create --ipv6 --subnet fc00:f853:ccd:e791::/64 -d bridge network2

KIND_NODES=$("${KIND_BIN}" get nodes --name "${KIND_CLUSTER_NAME}")
for n in $KIND_NODES; do
  docker network connect network2 "$n"
done

# remove the exclude-from-external-loadbalancers annotation
for node in $KIND_NODES; do
  kubectl label nodes $node node.kubernetes.io/exclude-from-external-load-balancers-
done
