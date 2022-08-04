#!/bin/bash

kubectl apply --server-side -f hack/kube-prometheus/manifests/setup
until kubectl get servicemonitors --all-namespaces ; do date; sleep 1; echo ""; done
kubectl apply -f hack/kube-prometheus/manifests/
echo "Waiting for prometheus pods to be running"
kubectl -n monitoring wait --for=condition=Ready --all pods --timeout 300s