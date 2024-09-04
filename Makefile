# Copyright 2023 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

CONTROLLER_SHORT = cso
CONTROLLER_NAME = cluster-stack-operator
IMAGE_PREFIX ?= ghcr.io/sovereigncloudstack

STAGING_IMAGE = $(CONTROLLER_SHORT)-staging
BUILDER_IMAGE = $(IMAGE_PREFIX)/$(CONTROLLER_SHORT)-builder
BUILDER_IMAGE_VERSION = $(shell cat .builder-image-version.txt)
HACK_TOOLS_BIN_VERSION = $(shell cat ./hack/tools/bin/version.txt)

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec
.DEFAULT_GOAL:=help
GOTEST ?= go test

##@ General



# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

#############
# Variables #
#############

# Certain aspects of the build are done in containers for consistency (e.g. protobuf generation)
# If you have the correct tools installed and you want to speed up development you can run
# make BUILD_IN_CONTAINER=false target
# or you can override this with an environment variable
BUILD_IN_CONTAINER ?= true

# Boiler plate for building Docker containers.
TAG ?= dev
ARCH ?= amd64
# Allow overriding the imagePullPolicy
PULL_POLICY ?= Always
# Build time versioning details.
LDFLAGS := $(shell hack/version.sh)

TIMEOUT := $(shell command -v timeout || command -v gtimeout)

# Directories
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
EXP_DIR := exp
TEST_DIR := test
BIN_DIR := bin
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/$(BIN_DIR)
export PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)
export GOBIN := $(abspath $(TOOLS_BIN_DIR))

# Files
WORKER_CLUSTER_KUBECONFIG ?= ".workload-cluster-kubeconfig.yaml"
MGT_CLUSTER_KUBECONFIG ?= ".mgt-cluster-kubeconfig.yaml"

# Kubebuilder.
export KUBEBUILDER_ENVTEST_KUBERNETES_VERSION ?= 1.29.3
# versions
CTLPTL_VERSION := 0.8.25

##@ Binaries
############
# Binaries #
############
# need in CI for releasing
CONTROLLER_GEN := $(abspath $(TOOLS_BIN_DIR)/controller-gen)
$(CONTROLLER_GEN): # Build controller-gen from tools folder.
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0

# need this in CI for releasing
KUSTOMIZE := $(abspath $(TOOLS_BIN_DIR)/kustomize)
kustomize: $(KUSTOMIZE) ## Build a local copy of kustomize
$(KUSTOMIZE): # Build kustomize from tools folder.
	go install sigs.k8s.io/kustomize/kustomize/v4@v4.5.7

TILT := $(abspath $(TOOLS_BIN_DIR)/tilt)
tilt: $(TILT) ## Build a local copy of tilt
$(TILT):
	@mkdir -p $(TOOLS_BIN_DIR)
	MINIMUM_TILT_VERSION=0.33.3 hack/ensure-tilt.sh

ENVSUBST := $(abspath $(TOOLS_BIN_DIR)/envsubst)
envsubst: $(ENVSUBST) ## Build a local copy of envsubst
$(ENVSUBST): # Build envsubst from tools folder.
	go install github.com/drone/envsubst/v2/cmd/envsubst@latest

SETUP_ENVTEST := $(abspath $(TOOLS_BIN_DIR)/setup-envtest)
setup-envtest: $(SETUP_ENVTEST) ## Build a local copy of setup-envtest
$(SETUP_ENVTEST): # Build setup-envtest from tools folder.
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@v0.0.0-20230620070423-a784ee78d04b

CTLPTL := $(abspath $(TOOLS_BIN_DIR)/ctlptl)
ctlptl: $(CTLPTL) ## Build a local copy of ctlptl
$(CTLPTL):
	curl -sSL https://github.com/tilt-dev/ctlptl/releases/download/v$(CTLPTL_VERSION)/ctlptl.$(CTLPTL_VERSION).linux.x86_64.tar.gz | tar xz -C $(TOOLS_BIN_DIR) ctlptl

KUBECTL := $(abspath $(TOOLS_BIN_DIR)/kubectl)


HELM := $(abspath $(TOOLS_BIN_DIR)/helm)
helm: $(HELM) ## Build a local copy of helm
$(HELM):
	curl -sSL https://get.helm.sh/helm-v3.15.1-linux-amd64.tar.gz | tar xz -C $(TOOLS_BIN_DIR) --strip-components=1 linux-amd64/helm
	chmod a+rx $(HELM)

MOCKERY := $(abspath $(TOOLS_BIN_DIR)/mockery)
mockery: $(MOCKERY) ## Download and extract mockery binary from github releases page
$(MOCKERY):
	curl -sSL https://github.com/vektra/mockery/releases/download/v2.32.4/mockery_2.32.4_Linux_x86_64.tar.gz | tar xz -C $(TOOLS_BIN_DIR)



mock_dir := $(shell dirname $$(grep -r -w -l -E "type Client interface" --exclude='Makefile' --exclude-dir=vendor))

.PHONY: mock
mock: $(MOCKERY)
	@for path in $(mock_dir); do \
	    echo "Running mockery for $$path"; \
	    $(MOCKERY) --all --recursive --dir=$$path --output=$$path/mocks; \
	done

.PHONY: mock-clean
mock-clean:
	@echo "Erasing the directory test/$(mock_test_dir)/mocks"
	@rm -rf $(mock_test_dir)/mocks
	@for path in $(mock_dir); do \
	    echo "Erasing this directory $$path/mocks"; \
	    rm -rf $$path/mocks; \
	done

go-binsize-treemap := $(abspath $(TOOLS_BIN_DIR)/go-binsize-treemap)
go-binsize-treemap: $(go-binsize-treemap) # Build go-binsize-treemap from tools folder.
$(go-binsize-treemap):
	go install github.com/nikolaydubina/go-binsize-treemap@v0.2.0

go-cover-treemap := $(abspath $(TOOLS_BIN_DIR)/go-cover-treemap)
go-cover-treemap: $(go-cover-treemap) # Build go-cover-treemap from tools folder.
$(go-cover-treemap):
	go install github.com/nikolaydubina/go-cover-treemap@v1.3.0


GOTESTSUM := $(abspath $(TOOLS_BIN_DIR)/gotestsum)
gotestsum: $(GOTESTSUM) # Build gotestsum from tools folder.
$(GOTESTSUM):
	go install gotest.tools/gotestsum@v1.10.0


all-tools: get-dependencies $(CTLPTL) $(SETUP_ENVTEST) $(ENVSUBST) $(KUSTOMIZE) $(CONTROLLER_GEN)
	echo 'done'

##@ Development
###############
# Development #
###############
env-vars-for-wl-cluster:
	@./hack/ensure-env-variables.sh GIT_PROVIDER_B64 GIT_ORG_NAME_B64 GIT_REPOSITORY_NAME_B64 EXP_CLUSTER_RESOURCE_SET CLUSTER_TOPOLOGY CLUSTER_NAME

.PHONY: delete-bootstrap-cluster
delete-bootstrap-cluster: $(CTLPTL)  ## Deletes Kind-dev Cluster
	$(CTLPTL) delete cluster kind-cso
	$(CTLPTL) delete registry kind-registry

.PHONY: cluster
cluster: get-dependencies $(CTLPTL) $(KUBECTL) ## Creates kind-dev Cluster
	@# Fail early. Background: After Tilt started, changing .envrc has no effect for processes
	@# started via Tilt. That's why this should fail early.
	./hack/kind-dev.sh

##@ Clean
#########
# Clean #
#########
.PHONY: clean
clean: ## Remove all generated files
	$(MAKE) clean-bin

.PHONY: clean-bin
clean-bin: ## Remove all generated helper binaries
	rm -rf $(BIN_DIR)
	rm -rf $(TOOLS_BIN_DIR)

.PHONY: clean-release
clean-release: ## Remove the release folder
	rm -rf $(RELEASE_DIR)

.PHONY: clean-release-git
clean-release-git: ## Restores the git files usually modified during a release
	git restore ./*manager_config_patch.yaml ./*manager_pull_policy.yaml

##@ Releasing
#############
# Releasing #
#############
## latest git tag for the commit, e.g., v0.3.10
RELEASE_TAG ?= $(shell git describe --abbrev=0 2>/dev/null)
# the previous release tag, e.g., v0.3.9, excluding pre-release tags
PREVIOUS_TAG ?= $(shell git tag -l | grep -E "^v[0-9]+\.[0-9]+\.[0-9]." | sort -V | grep -B1 $(RELEASE_TAG) | head -n 1 2>/dev/null)
RELEASE_DIR ?= out
RELEASE_NOTES_DIR := _releasenotes

$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)/

$(RELEASE_NOTES_DIR):
	mkdir -p $(RELEASE_NOTES_DIR)/

.PHONY: test-release
test-release:
	$(MAKE) set-manifest-image MANIFEST_IMG=$(IMAGE_PREFIX)/cso-staging MANIFEST_TAG=$(TAG) TARGET_RESOURCE="./config/default/manager_config_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./config/default/manager_pull_policy.yaml"
	$(MAKE) release-manifests

.PHONY: release-manifests
release-manifests: generate-manifests generate-go-deepcopy $(KUSTOMIZE) $(RELEASE_DIR) ## Builds the manifests to publish with a release
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/cso-infrastructure-components.yaml
	## Build cso-components (aggregate of all of the above).
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml

.PHONY: release
release: clean-release  ## Builds and push container images using the latest git tag for the commit.
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "Your local git repository contains uncommitted changes, use git clean before proceeding."; exit 1; fi
	git checkout "${RELEASE_TAG}"
	# Set the manifest image to the production bucket.
	$(MAKE) set-manifest-image MANIFEST_IMG=$(IMAGE_PREFIX)/cso MANIFEST_TAG=$(RELEASE_TAG) TARGET_RESOURCE="./config/default/manager_config_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./config/default/manager_pull_policy.yaml"
	## Build the manifests
	$(MAKE) release-manifests clean-release-git

.PHONY: release-notes
release-notes: $(RELEASE_NOTES_DIR) $(RELEASE_NOTES)
	go run ./hack/tools/release/notes.go --from=$(PREVIOUS_TAG) > $(RELEASE_NOTES_DIR)/$(RELEASE_TAG).md

##@ Images
##########
# Images #
##########

.PHONY: set-manifest-image
set-manifest-image:
	$(info Updating kustomize image patch file for default resource)
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG}:$(MANIFEST_TAG)"'@' $(TARGET_RESOURCE)

.PHONY: set-manifest-pull-policy
set-manifest-pull-policy:
	$(info Updating kustomize pull policy file for default resource)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' $(TARGET_RESOURCE)

##@ Binary
##########
# Binary #
##########
ALL_MANAGERS = core

.PHONY: managers
managers: $(addprefix manager-,$(ALL_MANAGERS)) ## Run all manager-* targets

manager-core: ## Build controller binary.
	go build -mod=vendor -trimpath -ldflags "$(LDFLAGS)" -o bin/manager cmd/main.go

run: ## Run a controller from your host.
	go run .cmd/main.go

##@ Testing
###########
# Testing #
###########
ARTIFACTS ?= _artifacts
$(ARTIFACTS):
	mkdir -p $(ARTIFACTS)/

$(MGT_CLUSTER_KUBECONFIG):
	./hack/get-kubeconfig-of-management-cluster.sh

$(WORKER_CLUSTER_KUBECONFIG):
	./hack/get-kubeconfig-of-workload-cluster.sh

KUBEBUILDER_ASSETS ?= $(shell $(SETUP_ENVTEST) use --use-env --bin-dir $(abspath $(TOOLS_BIN_DIR)) -p path $(KUBEBUILDER_ENVTEST_KUBERNETES_VERSION))

.PHONY: test-integration
test-integration: test-integration-workloadcluster  test-integration-github
	echo done

.PHONY: test-unit
test-unit: $(SETUP_ENVTEST) $(GOTESTSUM) $(HELM) ## Run unit
	@mkdir -p $(shell pwd)/.coverage
	CREATE_KIND_CLUSTER=false KUBEBUILDER_ASSETS="$(KUBEBUILDER_ASSETS)" $(GOTESTSUM) --junitfile=.coverage/junit.xml --format testname -- -mod=vendor \
	-covermode=atomic -coverprofile=.coverage/cover.out -p=4 ./internal/controller/...

.PHONY: test-integration-workloadcluster
test-integration-workloadcluster: $(SETUP_ENVTEST) $(GOTESTSUM)
	@mkdir -p $(shell pwd)/.coverage
	CREATE_KIND_CLUSTER=true KUBEBUILDER_ASSETS="$(KUBEBUILDER_ASSETS)" $(GOTESTSUM) --junitfile=.coverage/junit.xml --format testname -- -mod=vendor \
	-covermode=atomic -coverprofile=.coverage/cover.out -p=1  ./internal/test/integration/workloadcluster/...

.PHONY: test-integration-github
test-integration-github: $(SETUP_ENVTEST) $(GOTESTSUM)
	@mkdir -p $(shell pwd)/.coverage
	CREATE_KIND_CLUSTER=false KUBEBUILDER_ASSETS="$(KUBEBUILDER_ASSETS)" $(GOTESTSUM) --junitfile=.coverage/junit.xml --format testname -- -mod=vendor \
	-covermode=atomic -coverprofile=.coverage/cover.out -p=1  ./internal/test/integration/github/...

##@ Verify
##########
# Verify #
##########
.PHONY: verify-shellcheck
verify-shellcheck: ## Verify shell files
	./hack/verify-shellcheck.sh

.PHONY: verify-container-images
verify-container-images: ## Verify container images
	trivy image -q --exit-code 1 --ignore-unfixed --severity MEDIUM,HIGH,CRITICAL $(IMAGE_PREFIX)/$(CONTROLLER_SHORT):latest

##@ Generate
############
# Generate #
############
ALL_GENERATE_MODULES = core

# support go modules
generate-modules: ## Generates missing go modules
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run --rm  \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-stack-operator$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	./hack/golang-modules-update.sh
endif

generate-modules-ci: generate-modules
	@if ! (git diff --exit-code ); then \
		echo "\nChanges found in generated files"; \
		exit 1; \
	fi

.PHONY: generate-manifests
generate-manifests: $(addprefix generate-manifests-,$(ALL_GENERATE_MODULES)) ## Run all generate-manifests-* targets

generate-manifests-core: $(CONTROLLER_GEN) ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) \
			paths=./api/... \
			paths=./internal/controller/... \
			crd:crdVersions=v1 \
			rbac:roleName=manager-role \
			output:crd:dir=./config/crd/bases \
			output:webhook:dir=./config/webhook \
			webhook

.PHONY: generate-go-deepcopy
generate-go-deepcopy:  ## Run all generate-go-deepcopy-* targets
	$(MAKE) $(addprefix generate-go-deepcopy-,$(ALL_GENERATE_MODULES))

generate-go-deepcopy-core: $(CONTROLLER_GEN) ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) \
		object:headerFile="./hack/boilerplate/boilerplate.generatego.txt" \
		paths="./api/..."

generate-api-ci: generate-manifests-core generate-go-deepcopy
	@if ! (git diff --exit-code ); then \
		echo "\nChanges found in generated files"; \
		exit 1; \
	fi

##@ Format
##########
# Format #
##########
.PHONY: format-golang
format-golang: ## Format the Go codebase and run auto-fixers if supported by the linter.
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-stack-operator$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	go version
	golangci-lint version
	GO111MODULE=on golangci-lint run -v --fix
endif

.PHONY: format-yaml
format-yaml: ## Lint YAML files
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-stack-operator$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	yamlfixer --version
	yamlfixer -c .yamllint.yaml .
endif

##@ Lint
########
# Lint #
########

.PHONY: lint-golang
lint-golang: ## Lint Golang codebase
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-stack-operator$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	go version
	golangci-lint version
	GO111MODULE=on golangci-lint run -v
endif

.PHONY: lint-golang-ci
lint-golang-ci:
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-stack-operator$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	go version
	golangci-lint version
	GO111MODULE=on golangci-lint run -v --out-format=colored-line-number
endif

.PHONY: lint-yaml
lint-yaml: ## Lint YAML files
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-stack-operator$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	yamllint --version
	yamllint -c .yamllint.yaml --strict .
endif

.PHONY: lint-yaml-ci
lint-yaml-ci:
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-stack-operator$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	yamllint --version
	yamllint -c .yamllint.yaml . --format github
endif

DOCKERFILES=$(shell find . -not \( -path ./hack -prune \) -not \( -path ./vendor -prune \) -type f -regex ".*Dockerfile.*"  | tr '\n' ' ')
.PHONY: lint-dockerfile
lint-dockerfile: ## Lint Dockerfiles
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run  --rm -t -i \
		-v $(shell go env GOPATH)/pkg:/go/pkg$(MOUNT_FLAGS) \
		-v $(shell pwd):/src/cluster-stack-operator$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	hadolint --version
	hadolint -t error $(DOCKERFILES)
endif

lint-links: ## Link Checker
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run --rm -t -i \
		-v $(shell pwd):/src/cluster-stack-operator$(MOUNT_FLAGS) \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	lychee --config .lychee.toml ./*.md
endif

##@ Main Targets
################
# Main Targets #
################
.PHONY: lint
lint: lint-golang lint-yaml lint-dockerfile lint-links ## Lint Codebase

.PHONY: format
format: format-golang format-yaml ## Format Codebase

.PHONY: generate
generate: generate-manifests generate-go-deepcopy generate-modules ## Generate Files

ALL_VERIFY_CHECKS = shellcheck
.PHONY: verify
verify: generate lint $(addprefix verify-,$(ALL_VERIFY_CHECKS)) ## Verify all
	@if ! (git diff --exit-code ); then \
		echo "\nChanges found in generated files"; \
		echo "Please check the generated files and stage or commit the changes to fix this error."; \
		echo "If you are actively developing these files you can ignore this error"; \
		echo "(Don't forget to check in the generated files when finished)\n"; \
		exit 1; \
	fi

.PHONY: modules
modules: generate-modules ## Update go.mod & go.sum

.PHONY: builder-image-push
builder-image-push: ## Build $(CONTROLLER_SHORT)-builder to a new version. For more information see README.
	BUILDER_IMAGE=$(BUILDER_IMAGE) ./hack/upgrade-builder-image.sh

create-workload-cluster-docker: $(ENVSUBST) $(KUBECTL)
	cat .cluster.yaml | $(ENVSUBST) - | $(KUBECTL) apply -f -

.PHONY: tilt-up
tilt-up: env-vars-for-wl-cluster get-dependencies $(ENVSUBST) $(TILT) cluster  ## Start a mgt-cluster & Tilt. Installs the CRDs and deploys the controllers
	EXP_CLUSTER_RESOURCE_SET=true $(TILT) up --port=10351

BINARIES = clusterctl controller-gen helm kind kubectl kustomize trivy
get-dependencies:
ifeq ($(BUILD_IN_CONTAINER),true)
	docker run --rm -t -i \
		-v $(shell pwd):/src/cluster-stack-operator \
		$(BUILDER_IMAGE):$(BUILDER_IMAGE_VERSION) $@;
else
	@if [ "$(HACK_TOOLS_BIN_VERSION)" != "$(BUILDER_IMAGE_VERSION)" ]; then \
		echo "Updating binaries"; \
		rm -rf hack/tools/bin; \
		mkdir -p $(TOOLS_BIN_DIR); \
		cp ./.builder-image-version.txt $(TOOLS_BIN_DIR)/version.txt; \
		for tool in $(BINARIES); do \
			if command -v $$tool > /dev/null; then \
				cp `command -v $$tool` $(TOOLS_BIN_DIR); \
				echo "copied $$tool to $(TOOLS_BIN_DIR)"; \
			else \
				echo "$$tool not found"; \
			fi; \
		done; \
	else \
		echo "No action required"; \
		echo "Binaries are up to date"; \
	fi
endif
