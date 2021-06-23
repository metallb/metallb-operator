
# Image URL to use all building/pushing image targets
IMG ?= quay.io/metallb/metallb-operator:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,crdVersions=v1"
# Which dir to use in deploy kustomize build
KUSTOMIZE_DEPLOY_DIR ?= config/default

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

test: generate fmt vet manifests  ## Run tests
	go test ./... -coverprofile cover.out

test-e2e: generate fmt vet manifests  ## Run e2e tests
	go test --tags=e2etests -v ./test/e2e -ginkgo.v

manager: generate fmt vet  ## Build manager binary
	go build -o bin/manager main.go

run: generate fmt vet manifests  ## Run against the configured Kubernetes cluster in ~/.kube/config
	go run ./main.go

install: manifests kustomize  ## Install CRDs into a cluster
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall: manifests kustomize  ## Uninstall CRDs into a cluster
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: manifests kustomize  ## Deploy controller in the configured Kubernetes cluster in ~/.kube/config
	cd config/manager && kustomize edit set image controller=${IMG}
	$(KUSTOMIZE) build $(KUSTOMIZE_DEPLOY_DIR) | kubectl apply -f -
	$(KUSTOMIZE) build config/metallb_rbac | kubectl apply -f -

manifests: controller-gen  ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

fmt:  ## Run go fmt against code
	go fmt ./...

vet:  ## Run go vet against code
	go vet ./...

generate: controller-gen  ## Generate code
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

docker-build: test  ## Build the docker image
	docker build . -t ${IMG}

docker-push:  ## Push the docker image
	docker push ${IMG}

# download controller-gen if necessary
controller-gen:  ## Find or download controller-gen
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

generate-metallb-manifests:  ## Generate metallb manifests
	@echo "Generating MetalLB manifests"
	hack/generate-metallb-manifests.sh

validate-metallb-manifests:  ## Validate metallb manifests
	@echo "Comparing newly generated MetalLB manifests to existing ones"
	hack/compare-gen-manifests.sh

help:  ## Show this help
	@grep -F -h "##" $(MAKEFILE_LIST) | grep -F -v grep | sed -e 's/\\$$//' \
		| awk -F'[:#]' '{print $$1 = sprintf("%-30s", $$1), $$4}'