
# Image URL to use all building/pushing image targets
IMG ?= quay.io/metallb/metallb-operator:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,crdVersions=v1"
# Which dir to use in deploy kustomize build
KUSTOMIZE_DEPLOY_DIR ?= config/default

# Default bundle index image tag
BUNDLE_INDEX_IMG ?= $(REPO)/metallb-operator-bundle-index:$(VERSION)

# Default namespace
NAMESPACE ?= metallb-system

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

test-e2e: generate fmt vet manifests
	go test --tags=e2etests -v ./test/e2e -ginkgo.v

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize
	cd config/manager && kustomize edit set image controller=${IMG}
	$(KUSTOMIZE) build $(KUSTOMIZE_DEPLOY_DIR) | kubectl apply -f -
	$(KUSTOMIZE) build config/metallb_rbac | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build: test
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

deploy-olm:
	kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.18.2/crds.yaml
	kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.18.2/olm.yaml

deploy-with-olm:
	sed -i 's#quay.io/metallb/metallb-operator-bundle-index:latest#$(BUNDLE_INDEX_IMG)#g' config/olm-install/install-resources.yaml
	sed -i 's#mymetallb#$(NAMESPACE)#g' config/olm-install/install-resources.yaml
	$(KUSTOMIZE) build config/olm-install | kubectl apply -f -

# Build the bundle index image.
bundle-index-build: opm
	$(OPM) index add --bundles $(BUNDLE_IMG) --tag $(BUNDLE_INDEX_IMG) -c docker

# Generate and push bundle image and bundle index image
build-and-push-bundle-images:
	$(MAKE) bundle
	$(MAKE) build-bundle
	$(MAKE) docker-push IMG=$(BUNDLE_IMG)
	$(MAKE) bundle-index-build
	$(MAKE) docker-push IMG=$(BUNDLE_INDEX_IMG)

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

# Get the current opm binary. If there isn't any, we'll use the
# GOBIN path
opm:
ifeq (, $(shell which opm))
	@{ \
	set -e ;\
	opm_tool_url=https://api.github.com/repos/operator-framework/operator-registry/releases ;\
	opm_tool_latest_version=$$(curl -s $$opm_tool_url | grep tag_name | grep -v -- '-rc' | head -1 | awk -F': ' '{print $$2}' | sed 's/,//' | xargs) ;\
	curl -Lk https://github.com/operator-framework/operator-registry/releases/download/$$opm_tool_latest_version/linux-amd64-opm > $(GOBIN)/opm ;\
	chmod u+x $(GOBIN)/opm ;\
	}
OPM=$(GOBIN)/opm
else
OPM=$(shell which opm)
endif

generate-metallb-manifests:
	@echo "Generating MetalLB manifests"
	hack/generate-metallb-manifests.sh

validate-metallb-manifests:
	@echo "Comparing newly generated MetalLB manifests to existing ones"
	hack/compare-gen-manifests.sh
