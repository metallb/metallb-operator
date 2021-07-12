
# Current Operator version
VERSION ?= 0.1.0
# Default image repo
REPO ?= quay.io/metallb

# Image URL to use all building/pushing image targets
IMG ?= $(REPO)/metallb-operator:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,crdVersions=v1"
# Which dir to use in deploy kustomize build
KUSTOMIZE_DEPLOY_DIR ?= config/default

# Default bundle image tag
BUNDLE_IMG ?= $(REPO)/metallb-operator-bundle:$(VERSION)
# Default bundle index image tag
BUNDLE_INDEX_IMG ?= $(REPO)/metallb-operator-bundle-index:$(VERSION)

# Options for 'bundle'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# Default namespace
NAMESPACE ?= metallb-system

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

OPERATOR_SDK_URL=https://api.github.com/repos/operator-framework/operator-sdk/releases
OLM_URL=https://api.github.com/repos/operator-framework/operator-lifecycle-manager/releases
OPM_TOOL_URL=https://api.github.com/repos/operator-framework/operator-registry/releases

# Run tests
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: generate fmt vet manifests
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.8.3/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test ./... -coverprofile cover.out

all: manager ## Default make target if no options specified

test: generate fmt vet manifests  ## Run tests
	go test ./... -coverprofile cover.out

test-e2e: generate fmt vet manifests  ## Run e2e tests
	go test --tags=e2etests -v ./test/e2e -ginkgo.v

manager: generate fmt vet  ## Build manager binary
	go build -ldflags "-X main.build=$$(git rev-parse HEAD)" -o bin/manager main.go

run: generate fmt vet manifests  ## Run against the configured cluster
	go run ./main.go

install: manifests kustomize  ## Install CRDs into a cluster
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall: manifests kustomize  ## Uninstall CRDs from a cluster
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: manifests kustomize  ## Deploy controller in the configured cluster
	cd config/manager && kustomize edit set image controller=${IMG}
	$(KUSTOMIZE) build $(KUSTOMIZE_DEPLOY_DIR) | kubectl apply -f -
	$(KUSTOMIZE) build config/metallb_rbac | kubectl apply -f -

manifests: controller-gen  ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

fmt:  ## Run go fmt against code
	[ -z "`gofmt -s -w -l -e .`" ]
	go fmt ./...

vet:  ## Run go vet against code
	go vet ./...

generate: controller-gen  ## Generate code
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build:  ## Build the docker image
	docker build . -t ${IMG}

docker-push:  ## Push the docker image
	docker push ${IMG}

bundle: operator-sdk manifests ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests --interactive=false -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS) --extra-service-accounts "controller,speaker"
	$(OPERATOR_SDK) bundle validate ./bundle

build-bundle: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

deploy-olm:
	olm_latest_version=$$(curl -s $(OLM_URL) | grep tag_name | grep -v -- '-rc' | head -1 | awk -F': ' '{print $$2}' | sed 's/,//' | xargs) ;\
	kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/$${olm_latest_version}/crds.yaml ;\
	kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/$${olm_latest_version}/olm.yaml ;\

deploy-with-olm:
	sed -i 's#quay.io/metallb/metallb-operator-bundle-index:latest#$(BUNDLE_INDEX_IMG)#g' config/olm-install/install-resources.yaml
	sed -i 's#mymetallb#$(NAMESPACE)#g' config/olm-install/install-resources.yaml
	$(KUSTOMIZE) build config/olm-install | kubectl apply -f -

bundle-index-build: opm  ## Build the bundle index image.
	$(OPM) index add --bundles $(BUNDLE_IMG) --tag $(BUNDLE_INDEX_IMG) -c docker

build-and-push-bundle-images: docker-build docker-push  ## Generate and push bundle image and bundle index image
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

# Get the current operator-sdk binary. If there isn't any, we'll use the
# GOBIN path
operator-sdk:
ifeq (, $(shell which operator-sdk))
	@{ \
	set -e ;\
	operator_sdk_latest_version=$$(curl -s $(OPERATOR_SDK_URL) | grep tag_name | grep -v -- '-rc' | head -1 | awk -F': ' '{print $$2}' | sed 's/,//' | xargs) ;\
	curl -Lk  https://github.com/operator-framework/operator-sdk/releases/download/$$operator_sdk_latest_version/operator-sdk_linux_amd64 > $(GOBIN)/operator-sdk ;\
	chmod u+x $(GOBIN)/operator-sdk ;\
	}
OPERATOR_SDK=$(GOBIN)/operator-sdk
else
OPERATOR_SDK=$(shell which operator-sdk)
endif

# Get the current opm binary. If there isn't any, we'll use the
# GOBIN path
opm:
ifeq (, $(shell which opm))
	@{ \
	set -e ;\
	opm_tool_latest_version=$$(curl -s $(OPM_TOOL_URL) | grep tag_name | grep -v -- '-rc' | head -1 | awk -F': ' '{print $$2}' | sed 's/,//' | xargs) ;\
	curl -Lk https://github.com/operator-framework/operator-registry/releases/download/$$opm_tool_latest_version/linux-amd64-opm > $(GOBIN)/opm ;\
	chmod u+x $(GOBIN)/opm ;\
	}
OPM=$(GOBIN)/opm
else
OPM=$(shell which opm)
endif

generate-metallb-manifests:  ## Generate metallb manifests
	@echo "Generating MetalLB manifests"
	hack/generate-metallb-manifests.sh

validate-metallb-manifests:  ## Validate metallb manifests
	@echo "Comparing newly generated MetalLB manifests to existing ones"
	hack/compare-gen-manifests.sh

help:  ## Show this help
	@grep -F -h "##" $(MAKEFILE_LIST) | grep -F -v grep | sed -e 's/\\$$//' \
		| awk -F'[:#]' '{print $$1 = sprintf("%-30s", $$1), $$4}'