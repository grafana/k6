all: build

.PHONY: build
build: ## Build go package
	go build

.PHONY: format
format:
	find . -name '*.go' -exec gofmt -s -w {} +

.PHONY: check
check:
	golangci-lint run --out-format=tab --new-from-rev master ./...
	go test -race -timeout 210s ./...

.PHONY: container
container: ## Build docker container with `--rm`, `--pull`, `--no-cache` options
	docker build --rm --pull --no-cache -t loadimpact/k6 .

.PHONY: help
help: SHELL := /bin/sh
help: ## List available commands and their usage
	@awk 'BEGIN {FS = ":.*?##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[0-9a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } ' $(MAKEFILE_LIST)