# MetalLB Operator

This is the official MetalLB Operator, implementing the [operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
for deploying MetalLB on a kubernetes cluster, as described in the [related design proposal](https://github.com/metallb/metallb/blob/main/design/metallb-operator.md).

## Quick Setup

To install the MetalLB Operator using the prebuilt manifests, run the following:
```shell
kubectl apply -f bin/metallb-operator.yaml
```

## Installation

To install the MetalLB Operator using a prebuilt image, run the following:
```shell
make deploy
```

## Installation on OpenShift

To install the MetalLB Operator using a prebuilt image on OpenShift, run the following:
```shell
make deploy-openshift
```
> **Note:** This requires kustomize 4.5.6 or above.

> **Note:** To undeploy on OpenShift, use the `undeploy-openshift` target.

## Usage

Once the MetalLB Operator is installed, you have to create a `MetalLB` custom resource to deploy a MetalLB instance. The operator will consume this resource and create all required MetalLB resources based on it. The `MetalLB` custom resource needs to be created inside the `metallb-system` namespace and be named `metallb`. Only one `MetalLB` resource can exist in a cluster.

Following is a sample `MetalLB` resource:

```yaml
apiVersion: metallb.io/v1beta1
kind: MetalLB
metadata:
  name: metallb
  namespace: metallb-system
```

## Setting up a development environment

### Quick local installation

A quick, local installation can be done using a [kind](https://kind.sigs.k8s.io/) cluster and a local registry. Follow the steps below to run a locally-built metallb-operator on kind.

```shell
make deploy
```

### Create a MetalLB deployment

To create a MetalLB deployment, a MetalLB Operator configuration resource needs to be created. Run the following command to create it:

```shell
cat << EOF | kubectl apply -f -
apiVersion: metallb.io/v1beta1
kind: MetalLB
metadata:
  name: metallb
  namespace: metallb-system
EOF
```

### Running tests

To run metallb-operator unit tests (no cluster required), execute the following:

```shell
make test
```

To run metallb-operator e2e tests, execute the following:

```shell
make test-e2e
```

The e2e test need a running cluster with a MetalLB Operator deployed.

### Make

Most tasks in the project are automated using a Makefile.
Run `make help` to see the details.

## Releasing

The Operator assumes the same branching structure as MetalLB.
Each minor version must have a corresponding branch where we tag releases out.
Versioned branches must be pinned to specific MetalLB / Operator images.

The current version of the images is bumped under `hack/metallb_version.txt` and
under `hack/operator_version.txt`.

A convenience `make bump_versions` makefile target aligns the versions in the manifests to
the content of those files.

Another Makefile target `make fetch_metallb_version` updates `hack/metallb_version.txt` with the
latest tag of metallb.

In order to make a release, a tag must be made out of a release branch, pinning the relevant images.
