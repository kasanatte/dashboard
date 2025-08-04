# Basic variables
GOOS 					?= $(shell go env GOOS)
GOARCH 					?= $(shell go env GOARCH)
VERSION 				?= $(shell hack/version.sh)

# Default target when just running 'make'
.DEFAULT_GOAL := all

# Build targets
TARGETS := karmada-dashboard-api \
		   karmada-dashboard-web

# Docker image related variables
REGISTRY				?= docker.io/karmada
REGISTRY_USER_NAME  	?= 
REGISTRY_PASSWORD   	?= 
REGISTRY_SERVER_ADDRESS ?= 
IMAGE_TARGET := $(addprefix image-, $(TARGETS))

# Docker build variables
DOCKER_IMAGE_NAME ?= karmada-dashboard
DOCKER_IMAGE_TAG ?= $(VERSION)
DOCKER_BUILD_ARGS ?= --build-arg VERSION=$(VERSION)

# Development server variables
KARMADA_CTX ?= karmada-apiserver
HOST_CTX ?= karmada-host
API_PORT ?= 8000
API_HOST ?= http://localhost:8000
SKIP_TLS_VERIFY ?= false
KUBECONFIG ?= $(HOME)/.kube/karmada.config

###################
# Build Targets   #
###################

# Build all binaries (alias for build)
.PHONY: all
all: build

# Build all binaries
.PHONY: build
build: $(TARGETS)

# Build specific binary
.PHONY: $(TARGETS)
$(TARGETS):
	hack/build.sh $@

###################
# Docker Images   #
###################

# Build all images
.PHONY: images
images: $(IMAGE_TARGET)

# Build specific image
.PHONY: $(IMAGE_TARGET)
$(IMAGE_TARGET):
	set -e;\
	target=$$(echo $(subst image-,,$@));\
	make $$target GOOS=linux;\
	VERSION=$(VERSION) REGISTRY=$(REGISTRY) BUILD_PLATFORMS=linux/$(GOARCH) hack/docker.sh $$target

###################
# UI Building     #
###################

.PHONY: bundle-ui-dashboard
bundle-ui-dashboard:
	cd ui && pnpm run dashboard:build

###################
# Dependencies    #
###################

# Install Go dependencies
.PHONY: install-deps
install-deps:
	go mod tidy
	go mod verify

# Install UI dependencies
.PHONY: install-ui-deps
install-ui-deps:
	cd ui && pnpm install

# Install all dependencies
.PHONY: install
install: install-deps install-ui-deps

###################
# Development     #
###################

# Run both API and Web server
.PHONY: run
run:
ifndef KUBECONFIG
	$(error KUBECONFIG is required. Please specify the path to karmada kubeconfig)
endif
	@echo "Starting API server..."
	@_output/bin/$(GOOS)/$(GOARCH)/karmada-dashboard-api \
		--karmada-kubeconfig=$(KUBECONFIG) \
		--karmada-context=$(KARMADA_CTX) \
		--kubeconfig=$(KUBECONFIG) \
		--context=$(HOST_CTX) \
		--insecure-port=$(API_PORT) \
		$(if $(filter true,$(SKIP_TLS_VERIFY)),--skip-karmada-apiserver-tls-verify) & echo $$! > .api.pid
	@echo "Starting Web server..."
	@cd ui && VITE_API_HOST=$(API_HOST) pnpm run dashboard:dev || (kill `cat .api.pid` && rm .api.pid)
	@if [ -f .api.pid ]; then kill `cat .api.pid` && rm .api.pid; fi

# Run API server only
.PHONY: run-api
run-api: build
ifndef KUBECONFIG
	$(error KUBECONFIG is required. Please specify the path to karmada kubeconfig)
endif
	_output/bin/$(GOOS)/$(GOARCH)/karmada-dashboard-api \
		--karmada-kubeconfig=$(KUBECONFIG) \
		--karmada-context=$(KARMADA_CTX) \
		--kubeconfig=$(KUBECONFIG) \
		--context=$(HOST_CTX) \
		--insecure-port=$(API_PORT) \
		$(if $(filter true,$(SKIP_TLS_VERIFY)),--skip-karmada-apiserver-tls-verify)

# Run Web server only
.PHONY: run-web
run-web: install-ui-deps build
	cd ui && VITE_API_HOST=$(API_HOST) pnpm run dashboard:dev

# Generate JWT token for dashboard login
.PHONY: gen-token
gen-token:
ifndef KUBECONFIG
	$(error KUBECONFIG is required. Please specify the path to karmada kubeconfig)
endif
	@echo "Switching to karmada-apiserver context..."
	@kubectl config use-context $(KARMADA_CTX)
	@echo "Creating service account..."
	@kubectl apply -f artifacts/dashboard/karmada-dashboard-sa.yaml
	@echo "Getting JWT token..."
	@kubectl -n karmada-system get secret/karmada-dashboard-secret -o go-template="{{.data.token | base64decode}}"
	@echo

###################
# Docker Commands #
###################

# Build Docker image with integrated MCP server
.PHONY: docker-build
docker-build:
	docker build -t $(REGISTRY)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) $(DOCKER_BUILD_ARGS) .

# Run Docker container with integrated MCP server
.PHONY: docker-run
docker-run:
ifndef KUBECONFIG
	$(error KUBECONFIG is required. Please specify the path to karmada kubeconfig)
endif
	docker run -it --rm \
		-p 8000:8000 \
		-v $(KUBECONFIG):/home/karmada/.kube/karmada.config:ro \
		-e KUBECONFIG=/home/karmada/.kube/karmada.config \
		$(REGISTRY)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)

# Run with docker-compose
.PHONY: docker-compose
docker-compose:
	docker-compose up --build

# Stop docker-compose
.PHONY: docker-compose-down
docker-compose-down:
	docker-compose down

# Push Docker image
.PHONY: docker-push
docker-push: docker-build
	docker push $(REGISTRY)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)

###################
# Helm chart      #
###################
.PHONY: package-chart
package-chart:
	hack/package-helm-chart.sh $(VERSION)

.PHONY: push-chart
push-chart:
	helm push _output/charts/karmada-dashboard-chart-${VERSION}.tgz oci://docker.io/karmada

###################
# Help            #
###################

# Show help
.PHONY: help
help:
	@echo "Karmada Dashboard Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make                  - Build all binaries (same as 'make all')"
	@echo "  make all              - Build all binaries"
	@echo "  make install          - Install all dependencies"
	@echo "  make run              - Run both API and Web servers"
	@echo "  make images           - Build all Docker images"
	@echo ""
	@echo "Docker Commands:"
	@echo "  make docker-build     - Build Docker image with integrated MCP server"
	@echo "  make docker-run       - Run Docker container with integrated MCP server"
	@echo "  make docker-compose   - Run with docker-compose"
	@echo ""
	@echo "Development Commands:"
	@echo "  make run-api          - Run API server only"
	@echo "  make run-web          - Run Web server only"
	@echo "  make install-deps     - Install Go dependencies"
	@echo "  make install-ui-deps  - Install UI dependencies"
	@echo "  make gen-token        - Generate JWT token for dashboard login"
	@echo ""
	@echo "Build Commands:"
	@echo "  make build            - Build all binaries"
	@echo "  make images           - Build all Docker images"
	@echo ""
	@echo "Variables:"
	@echo "  KUBECONFIG            - Path to karmada kubeconfig file (default: $(HOME)/.kube/karmada.config)"
	@echo "  KARMADA_CTX           - Karmada API server context (default: karmada-apiserver)"
	@echo "  HOST_CTX              - Host cluster context (default: karmada-host)"
	@echo "  API_PORT              - API server port (default: 8000)"
	@echo "  API_HOST              - API server host (default: http://localhost:8000)"
	@echo "  GOOS                  - Target OS for build"
	@echo "  GOARCH                - Target architecture for build"
	@echo "  SKIP_TLS_VERIFY       - Skip TLS verification for Karmada API server (default: false)"
	@echo "  DOCKER_IMAGE_NAME     - Docker image name (default: karmada-dashboard)"
	@echo "  DOCKER_IMAGE_TAG      - Docker image tag (default: $(VERSION))"
	@echo "  REGISTRY              - Docker registry (default: docker.io/karmada)"
