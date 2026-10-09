IMAGE     ?= mukezhz/pay-np
TAG       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
PLATFORMS ?= linux/amd64,linux/arm64
PORT      ?= 8080
ENV_FILE  ?= .env

.DEFAULT_GOAL := help
.PHONY: help fmt vet test build run docker-build docker-run docker-push release

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

docker-push: ## Build for $(PLATFORMS) and push $(IMAGE):$(TAG) (CI owns latest/develop)
	docker buildx build --platform $(PLATFORMS) -t $(IMAGE):$(TAG) --push .

release: ## Tag VERSION (e.g. make release VERSION=v0.1.0) and push it; CI builds the release and image
	@test -n "$(VERSION)" || { echo "usage: make release VERSION=v0.1.0"; exit 1; }
	@echo "$(VERSION)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$$' || { echo "VERSION must look like v1.2.3"; exit 1; }
	@test -z "$$(git status --porcelain)" || { echo "working tree not clean"; exit 1; }
	git tag -a $(VERSION) -m "$(VERSION)"
	git push origin $(VERSION)
