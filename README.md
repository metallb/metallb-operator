# MetalLB Operator

This is a WIP implementaton of a MetalLB Operator, implementing the [operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
for deploying MetalLB on a kubernetes cluster, as described in the [related design proposal](https://github.com/metallb/metallb/blob/main/design/metallb-operator.md).

## Prerequisites

Need to install the following packages:

- operator-sdk 1.8.0+
- controller-gen v0.7.0+

To install controller-gen, run the following:

```
go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.7.0
```

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

**Install and run kind**

Install kind using the instructions [here](https://kind.sigs.k8s.io/docs/user/quick-start/).

Once kind is installed, run the following to start a kind cluster:

```shell
kind create cluster
kind get kubeconfig > kubeconfig
export KUBECONFIG=$(pwd)/kubeconfig
```

**Build and deploy the operator**

To build and deploy the operator, run the following:

```shell
export IMAGE_NAME=metallb-operator

make docker-build IMG=$IMAGE_NAME
kind load docker-image $IMAGE_NAME
IMG=$IMAGE_NAME KUSTOMIZE_DEPLOY_DIR="config/kind-ci/" make deploy
```

Alternatively, the image can be pushed to a Docker registry.

### Building and deploying using a remote repo

To build and push an image, run the following commands, specifying the preferred image repository and image:

```shell
make docker-build IMG=quay.io/example/metallboperator
make docker-push IMG=quay.io/example/metallboperator
```

Once the images are pushed to the repo, you can deploy MetalLB using your custom images by running the following:

```shell
make deploy IMG=quay.io/example/metallboperator
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
