#!/bin/bash

docker network create --ipv6 --subnet fc00:f853:ccd:e791::/64 -d bridge network2

KIND_NODES=$(kind get nodes --name "${KIND_CLUSTER_NAME}")
for n in $KIND_NODES; do
  docker network connect network2 "$n"
done