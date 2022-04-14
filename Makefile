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

all: build

build:
	go build

format:
	$(call run_k6_tools,gofumpt -w .)

# lint files
lint:
	$(call run_k6_tools,golangci-lint run --out-format=tab --new-from-rev master ./...)

# fix apply all possible auto-fixes
fix:
	$(call run_k6_tools,golangci-lint run --fix --new-from-rev master ./...)

# generates files like easyjson, enum, etc
generate:
	$(call run_k6_tools,go generate ./...)

tests:
	go test -race -timeout 210s ./...

check: lint tests

# enter into the k6 tools container
k6-dev-tools:
	$(call run_k6_tools,bash)

# builds the container with all required dev-tools
build-k6-dev-tools:
	docker build \
		--build-arg USER=$(USER) \
		--build-arg UID=$(shell id -u) \
		--build-arg GID=$(shell id -g) \
		--build-arg GOLANGCI_LINT_VERSION=$(GOLANGCI_LINT_VERSION) \
		-f Dockerfile.dev \
		--tag $(K6_DEV_TOOLS_IMAGE) .

.PHONY: build format lint tests check build-k6-dev-tools generate fix
