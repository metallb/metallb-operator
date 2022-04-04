SHELL := /bin/bash

# Current Operator version
VERSION ?= latest
CSV_VERSION = $(shell echo $(VERSION) | sed 's/v//')
ifeq ($(VERSION), latest)
CSV_VERSION := 0.0.0
endif
# Default image repo
REPO ?= quay.io/metallb

# Image URL to use all building/pushing image targets
IMG ?= $(REPO)/metallb-operator:$(VERSION)
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:crdVersions=v1"
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

OPERATOR_SDK_VERSION=v1.8.1
OLM_VERSION=v0.18.3
CERT_MANAGER_VERSION=v1.5.4

OPM_TOOL_URL=https://api.github.com/repos/operator-framework/operator-registry/releases

TESTS_REPORTS_PATH ?= /tmp/test_e2e_logs/
VALIDATION_TESTS_REPORTS_PATH ?= /tmp/test_validation_logs/

ENABLE_WEBHOOK ?= true

ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: generate fmt vet manifests ## Run unit and integration tests
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.8.3/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test ./... -coverprofile cover.out

all: manager ## Default make target if no options specified

test-validation: generate fmt vet manifests  ## Run validation tests
	rm -rf ${VALIDATION_TESTS_REPORTS_PATH}
	mkdir -p ${VALIDATION_TESTS_REPORTS_PATH}
	USE_LOCAL_RESOURCES=true go test --tags=validationtests -v ./test/e2e/validation -ginkgo.v -junit $(VALIDATION_TESTS_REPORTS_PATH) -report $(VALIDATION_TESTS_REPORTS_PATH)

test-functional: generate fmt vet manifests  ## Run e2e tests
	rm -rf ${TESTS_REPORTS_PATH}
	mkdir -p ${TESTS_REPORTS_PATH}
	USE_LOCAL_RESOURCES=true go test --tags=e2etests -v ./test/e2e/functional -ginkgo.v -junit $(TESTS_REPORTS_PATH) -report $(TESTS_REPORTS_PATH)

test-e2e: generate fmt vet manifests test-validation test-functional  ## Run e2e tests

manager: generate fmt vet  ## Build manager binary
	go build -ldflags "-X main.build=$$(git rev-parse HEAD)" -o bin/manager main.go

run: generate fmt vet manifests  ## Run against the configured cluster
	go run ./main.go

install: manifests kustomize  ## Install CRDs into a cluster
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall: manifests kustomize  ## Uninstall CRDs from a cluster
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

configure-webhook:
	ENABLE_WEBHOOK=$(ENABLE_WEBHOOK) hack/configure_webhook.sh

deploy-cert-manager: ## Deploys cert-manager. Fetching from https://github.com/jetstack/cert-manager
	set -e ;\
	kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml ;\
	hack/wait_for_cert_manager.sh ;\

deploy: manifests kustomize configure-webhook ## Deploy controller in the configured cluster
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	cd $(KUSTOMIZE_DEPLOY_DIR) && $(KUSTOMIZE) edit set namespace $(NAMESPACE)
	cd config/metallb_rbac && $(KUSTOMIZE) edit set namespace $(NAMESPACE)
	$(KUSTOMIZE) build $(KUSTOMIZE_DEPLOY_DIR) | kubectl apply -f -
	$(KUSTOMIZE) build config/metallb_rbac | kubectl apply -f -

undeploy: ## Undeploy the controller from the configured cluster
	$(KUSTOMIZE) build $(KUSTOMIZE_DEPLOY_DIR) | kubectl delete --ignore-not-found=true -f -
	$(KUSTOMIZE) build config/metallb_rbac | kubectl delete --ignore-not-found=true -f -

BIN_FILE ?= "metallb-operator.yaml"
bin: manifests kustomize configure-webhook ## Create manifests
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	cd $(KUSTOMIZE_DEPLOY_DIR) && $(KUSTOMIZE) edit set namespace $(NAMESPACE)
	cd config/metallb_rbac && $(KUSTOMIZE) edit set namespace $(NAMESPACE)
	$(KUSTOMIZE) build $(KUSTOMIZE_DEPLOY_DIR) > bin/$(BIN_FILE)
	echo "---" >> bin/$(BIN_FILE)
	$(KUSTOMIZE) build config/metallb_rbac >> bin/$(BIN_FILE)

manifests: controller-gen generate-metallb-manifests  ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role paths="./..." output:crd:artifacts:config=config/crd/bases

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

bundle: operator-sdk manifests configure-webhook ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests --interactive=false -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle -q --overwrite --version $(CSV_VERSION) $(BUNDLE_METADATA_OPTS) --extra-service-accounts "controller,speaker"
	$(OPERATOR_SDK) bundle validate ./bundle

bundle-release: bundle bump_versions ## Generate the bundle manifests for a PR

build-bundle: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

deploy-olm: operator-sdk ## deploys OLM on the cluster
	operator-sdk olm install --version $(OLM_VERSION)
	operator-sdk olm status

deploy-with-olm: ## deploys the operator with OLM instead of manifests
	sed -i 's#quay.io/metallb/metallb-operator-bundle-index:$(VERSION)#$(BUNDLE_INDEX_IMG)#g' config/olm-install/install-resources.yaml
	sed -i 's#mymetallb#$(NAMESPACE)#g' config/olm-install/install-resources.yaml
	$(KUSTOMIZE) build config/olm-install | kubectl apply -f -
	VERSION=$(CSV_VERSION) NAMESPACE=$(NAMESPACE) hack/wait-for-csv.sh

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
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.7.0 ;\
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
	go get sigs.k8s.io/kustomize/kustomize/v4@v4.4.0 ;\
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
	curl -Lk  https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_linux_amd64 > $(GOBIN)/operator-sdk ;\
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

generate-metallb-manifests:  ## Generate MetalLB manifests
	@echo "Generating MetalLB manifests"
	hack/generate-metallb-manifests.sh

validate-metallb-manifests:  ## Validate MetalLB manifests
	@echo "Comparing newly generated MetalLB manifests to existing ones"
	hack/compare-gen-manifests.sh

lint: ## Run golangci-lint against code
	@echo "Running golangci-lint"
	hack/lint.sh

fetch_metallb_version: ## Updates the versions of metallb under hack/metallb_version with the latest available tag
	@echo "Bumping metallb to latest"
	hack/fetch_latest_metallb.sh

bump_versions: ## Updates the versions of the metallb-operator / metallb image with the content of hack/operator_version / metallb_version
	@echo "Updating the operator version"
	hack/bump_versions.sh

check_generated: ## Checks if there are any different with the current checkout
	@echo "Checking generated files"
	hack/verify_generated.sh

help:  ## Show this help
	@grep -F -h "##" $(MAKEFILE_LIST) | grep -F -v grep | sed -e 's/\\$$//' \
		| awk -F'[:#]' '{print $$1 = sprintf("%-30s", $$1), $$4}'
