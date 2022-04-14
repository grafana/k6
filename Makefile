GOLANGCI_LINT_VERSION = $(shell head -n 1 .golangci.yml | tr -d '\# ')
K6_DEV_TOOLS_CONTAINER = k6-dev-tools

# todo: check if the image exists and if not suggest to run build-k6-dev-tools
# todo: implement validation if tool inside isn't outdated
define run_k6_tools
	@docker run --rm -t \
		--user "$(shell id -u $(USER))" \
		-v $(shell pwd):/app \
		-w /app $(K6_DEV_TOOLS_CONTAINER) \
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

# builds the container with all required dev-tools
build-k6-dev-tools:
	docker build \
		--build-arg USER=$(USER) \
		--build-arg UID=$(shell id -u) \
		--build-arg GID=$(shell id -g) \
		--build-arg GOLANGCI_LINT_VERSION=$(GOLANGCI_LINT_VERSION) \
		-f Dockerfile.dev \
		--tag $(K6_DEV_TOOLS_CONTAINER) .

.PHONY: build format lint tests check build-k6-dev-tools generate fix
