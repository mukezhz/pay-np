IMAGE     ?= ghcr.io/mukezhz/pay-np-checkout
TAG       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
PLATFORMS ?= linux/amd64,linux/arm64
PORT      ?= 8080
ENV_FILE  ?= .env

.DEFAULT_GOAL := help
.PHONY: help fmt vet test build run docker-build docker-run docker-push

help: ## Show targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

fmt: ## Format Go code
	gofmt -w .

vet: ## Run go vet
	go vet ./...

test: ## Run tests with the race detector
	go test -race ./...

build: ## Build the example checkout server into bin/checkout
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/checkout ./examples/checkout

run: ## Run the example checkout server locally
	go run ./examples/checkout

docker-build: ## Build $(IMAGE):$(TAG) for the local platform
	docker build -t $(IMAGE):$(TAG) -t $(IMAGE):latest .

docker-run: docker-build ## Run the image on $(PORT), reading $(ENV_FILE) if present
	docker run --rm -p $(PORT):8080 -e BASE_URL=http://localhost:$(PORT) \
		$(if $(wildcard $(ENV_FILE)),--env-file $(ENV_FILE)) $(IMAGE):$(TAG)

docker-push: ## Build for $(PLATFORMS) and push $(IMAGE):$(TAG) and :latest (docker login first)
	docker buildx build --platform $(PLATFORMS) -t $(IMAGE):$(TAG) -t $(IMAGE):latest --push .
