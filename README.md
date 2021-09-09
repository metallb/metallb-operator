# MetalLB Operator

This is a WIP implementaton of a MetalLB Operator, implementing the [operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
for deploying MetalLB on a kubernetes cluster, as described in the [related design proposal](https://github.com/metallb/metallb/blob/main/design/metallb-operator.md).

## Note that this is still work in progress and not ready for production by any means


## Prerequisites
Need to install the following packages
- operator-sdk 1.8.0+
- controller-gen v0.3.0+
```
     go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0
```

## Installation

To install the MetalLB Operator using a prebuilt image, run: 

```shell
make deploy
```

## Usage

Once the MetalLB Operator is installed, you have to create a `MetalLB` custom resource to install MetalLB. The operator will consume this resource, and create all required MetalLB resources based on it. The `MetalLB` custom resource needs to be created inside the `metallb-system` namespace and be named `metallb`. Only one `MetalLB` resource can exist in a cluster.

Below you can find an example of a `MetalLB` resource definition:

```yaml
apiVersion: metallb.io/v1beta1
kind: MetalLB
metadata:
  name: metallb
  namespace: metallb-system
```


## Setting up a development environment

### Quick local installation

A quick, local installation can be done using a kind cluster using a local registry. Follow the steps below to run a locally built metallb-operator on kind.

**Install and run kind**

 Read more about kind [here](https://kind.sigs.k8s.io/docs/user/quick-start/).
Once kind is installed please execute the following commands to start a kind cluster:

```shell
kind create cluster
kind get kubeconfig > kubeconfig
export KUBECONFIG=kubeconfig
```

**Build and deploy the operator**

Follow the steps below to build and deploy the operator.

```shell
export built_image=<image-name> 
# For example: export built_image=metallb-operator

 make docker-build IMG=$built_image
kind load docker-image $built_image
IMG=$built_image KUSTOMIZE_DEPLOY_DIR="config/kind-ci/" make deploy
```

Alternatively the image can be pushed to 

### Building and deploying using a remote repo

To build and push an image run the following commands, specifying the prefered image repository and image:

```shell
make docker-build IMG=<your image>
make docker-push IMG=<your image>
```

For example:

```shell
make docker-build IMG=quay.io/example/metalllboperator
make docker-push IMG=quay.io/example/metalllboperator
```

Once the images are pushed to the repo, you can deploy MetalLB using your custom images by running:
```shell
make deploy IMG=<your image>

```

### Create a MetalLB deployment

To create a MetalLB deployment, a MetalLB Operator configuration resource needs to be created.
Run the following command to create it:

```shell
cat << EOF | kubectl apply -f -
apiVersion: metallb.io/v1beta1
kind: MetalLB
metadata:
  name: metallb
  namespace: metallb-system
EOF
```

### Create an Address Pool object

To create an address pool, an AddressPool resource needs to be created.
An example of an AddressPool resource is shown below:

```yaml
apiVersion: metallb.io/v1alpha1
kind: AddressPool
metadata:
  name: addresspool-sample1
  namespace: metallb-system
spec:
  protocol: layer2
  addresses:
    - 172.18.0.100-172.18.0.255
```

When the address pool is successfully added, it will be amended to the `config` ConfigMap used to configure MetalLB:

```yaml
kind: ConfigMap
apiVersion: v1
data:
  config: |
    address-pools:
    - name: addresspool-sample1
      protocol: layer2
      addresses:
      - 172.18.0.100-172.18.0.255
```

### Create a BGP Peer object

To create a BGP peer, a BGPPeer resource needs to be created.
An example of a BGPPeer resource is shown below:

```yaml
apiVersion: metallb.io/v1alpha1
kind: BGPPeer
metadata:
  name: peer-sample1
  namespace: metallb-system
spec:
  peerAddress: 10.0.0.1
  peerASN: 64501
  myASN: 64500
  routerID: 10.10.10.10
  peerPort: 1
  holdTime: 10
  sourceAddress: "1.1.1.1"
  password: "test"
  nodeSelectors:
  - matchExpressions:
    - key: kubernetes.io/hostname
      operator: In
      values: [hostA, hostB]
```

### Sample MetalLB BGP configuration

```yaml
apiVersion: metallb.io/v1alpha1
kind: AddressPool
metadata:
  name: addresspool-bgp-sample
  namespace: metallb-system
spec:
  protocol: bgp
  addresses:
    - 172.18.0.100-172.18.0.255
---
apiVersion: metallb.io/v1alpha1
kind: BGPPeer
metadata:
  name: peer-sample
  namespace: metallb-system
spec:
  peerAddress: 10.0.0.1
  peerASN: 64501
  myASN: 64500
  routerID: 10.10.10.10
```

### Running tests

To run metallb-operator unit tests (no cluster required), execute:

```shell
make test
```

To run metallb-operator e2e tests, execute:

```shell
make test-e2e
```
The e2e test need a running cluster with a MetalLB Operator running.


### Make

Most tasks in the project are automated using a Makefile.
Please run `make help` to see the details.

## Releasing

The Operator assumes the same branching structure of MetalLB.
Each minor version must have a corresponding branch where we tag releases out.
Versioned branches must be pinned to specific metallb / operator images.

The current version of the images is bumped under `hack/metallb_version.txt` and
under `hack/operator_version.txt`.

A convenience `make bump_versions` makefile rule will align the versions in the manifests to
the content of those files.

Another makefile rule `make fetch_metallb_version` will update `hack/metallb_version.txt` with the
highest tag of metallb.

In order to make a release, a tag must be done out of a release branch, pinning the relevant images.