# MetalLB Operator

This is a WIP implementaton of a MetalLB operator, implementing the [operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
for deploying MetalLB on a kubernetes cluster, as described in the [related design proposal](https://github.com/metallb/metallb/blob/main/design/metallb-operator.md).

## Note that this is still work in progress and not ready for production by any means

## Prerequisites
Need to install the following packages
- operator-sdk 1.8.0+
- controller-gen v0.3.0+
```
     go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0
```