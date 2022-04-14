MAKEFLAGS += --silent
GOLANGCI_LINT_VERSION = $(shell head -n 1 .golangci.yml | tr -d '\# ')
TMPDIR ?= /tmp
K6_DEV_TOOLS_IMAGE = k6-dev-tools

# TODO: check if the image exists and if not suggest to run build-k6-dev-tools
# TODO: implement validation if tool inside isn't outdated
# TODO: a better cache key (maybe a image id)
define run_k6_tools
	@mkdir -p $(TMPDIR)/k6-dev-cache-$(GOLANGCI_LINT_VERSION)
	@docker run --rm -it \
		--user "$(shell id -u $(USER))" \
		-v $(TMPDIR)/k6-dev-cache-$(GOLANGCI_LINT_VERSION):/golangci-cache \
		--env "GOLANGCI_LINT_CACHE=/golangci-cache" \
		-v $(shell pwd):/app \
		-w /app $(K6_DEV_TOOLS_IMAGE) \
		$(1)
endef

all: build check

## help: Prints a list of available build targets.
help:
	echo "Usage: make <OPTIONS> ... <TARGETS>"
	echo ""
	echo "Available targets are:"
	echo ''
	sed -n 's/^##//p' ${PWD}/Makefile | column -t -s ':' | sed -e 's/^/ /'
	echo
	echo "Targets run by default are: `sed -n 's/^all: //p' ./Makefile | sed -e 's/ /, /g' | sed -e 's/\(.*\), /\1, and /'`"

## clean: Removes any previously created build artifacts.
clean:
	rm -f ./k6

## build: Builds the 'k6' binary.
build:
	go build

## format: Applies Go formatting to code.
format:
	$(call run_k6_tools,gofumpt -w .)

## fix: Runs golangci-lint with the verson that used inside the CI.
lint:
	$(call run_k6_tools,golangci-lint run --out-format=tab --new-from-rev master ./...)

## fix: Applies all possible auto-fixes that are detected by golangci-lint.
fix:
	$(call run_k6_tools,golangci-lint run --fix --new-from-rev master ./...)

## generate: Generates code, e.g. easyjson, enum, etc
generate:
	$(call run_k6_tools,go generate ./...)

## test: Executes any unit tests.
test:
	go test -race -timeout 210s ./...

## check: Performs most common checks like linting and unit testing.
check: lint tests

## k6-dev-tools: Enters into the container.
k6-dev-tools:
	$(call run_k6_tools,bash)

## build-k6-dev-tools: Builds the container with all tools for the development.
build-k6-dev-tools:
	docker build \
		--build-arg USER=$(USER) \
		--build-arg UID=$(shell id -u) \
		--build-arg GID=$(shell id -g) \
		--build-arg GOLANGCI_LINT_VERSION=$(GOLANGCI_LINT_VERSION) \
		-f Dockerfile.dev \
		--tag $(K6_DEV_TOOLS_IMAGE) .

.PHONY: build format lint test check build-k6-dev-tools k6-dev-tools generate fix
