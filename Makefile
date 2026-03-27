BINARY := odh-chaos
PKG := github.com/opendatahub-io/odh-platform-chaos
CMD := ./cmd/odh-chaos

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X $(PKG)/internal/cli.Version=$(VERSION)

# Container image settings
CONTAINER_TOOL ?= podman
IMAGE_REGISTRY ?= quay.io/opendatahub
IMAGE_NAME ?= odh-chaos
IMAGE_TAG ?= $(VERSION)
IMAGE ?= $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

# Dashboard image settings
DASHBOARD_IMAGE_NAME ?= chaos-dashboard
DASHBOARD_IMAGE ?= $(IMAGE_REGISTRY)/$(DASHBOARD_IMAGE_NAME):$(IMAGE_TAG)

# Code generation tools
CONTROLLER_GEN ?= go run sigs.k8s.io/controller-tools/cmd/controller-gen

.PHONY: build test test-short lint clean install \
	container-build container-push \
	dashboard-build dashboard-container-build dashboard-container-push \
	generate manifests deploy undeploy

##@ General

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-30s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

##@ Code Generation

generate: ## Generate deepcopy methods
	$(CONTROLLER_GEN) object paths=./api/...

manifests: ## Generate CRD manifests
	$(CONTROLLER_GEN) crd paths=./api/... output:crd:dir=config/crd/bases

##@ Build

build: ## Build the CLI binary
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(CMD)

dashboard-build: ## Build the dashboard binary (requires npm run build first)
	cd dashboard/ui && npm ci && npm run build
	go build -o bin/chaos-dashboard ./dashboard/cmd/dashboard/

install: build ## Install CLI to GOPATH
	cp bin/$(BINARY) $(GOPATH)/bin/

##@ Test

test: ## Run all tests with race detector
	go test -race ./... -v -count=1

test-short: ## Run tests without race detector
	go test ./... -short -count=1

lint: ## Run golangci-lint
	golangci-lint run ./...

##@ Container Images

container-build: ## Build CLI container image
	$(CONTAINER_TOOL) build --build-arg VERSION=$(VERSION) --build-arg TARGETARCH=$(shell go env GOARCH) -t $(IMAGE) -f Containerfile .

container-push: ## Push CLI container image
	$(CONTAINER_TOOL) push $(IMAGE)

dashboard-container-build: ## Build dashboard container image
	$(CONTAINER_TOOL) build --build-arg VERSION=$(VERSION) --build-arg TARGETARCH=$(shell go env GOARCH) -t $(DASHBOARD_IMAGE) -f dashboard/Containerfile .

dashboard-container-push: ## Push dashboard container image
	$(CONTAINER_TOOL) push $(DASHBOARD_IMAGE)

##@ Deployment

deploy: manifests ## Deploy controller and dashboard to cluster (requires kustomize)
	kubectl apply -k config/default/

undeploy: ## Remove controller and dashboard from cluster
	kubectl delete -k config/default/ --ignore-not-found

##@ Cleanup

clean: ## Remove build artifacts
	rm -rf bin/
