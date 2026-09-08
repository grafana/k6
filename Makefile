MAKEFLAGS += --silent
PROTOC_VERSION := 21.12

LINT_WORKFLOW ?= .github/workflows/lint.yml
K6_CI_REF     := $(shell grep -oE 'grafana/k6-ci/[^@[:space:]]+@[A-Za-z0-9._/-]+' $(LINT_WORKFLOW) | head -n1 | cut -d@ -f2)
LINT_BASE_URL := https://raw.githubusercontent.com/grafana/k6-ci/$(K6_CI_REF)/.golangci.yml
LINT_DIR      ?= build/lint
LINT_BASE     := $(LINT_DIR)/.golangci-base.yml
LINT_FINAL    := $(LINT_DIR)/.golangci.yml
LINT_PATCH    ?= .golangci.patch
GOLANGCI_LINT_VERSION = $(shell head -n 1 $(LINT_BASE) 2>/dev/null | tr -d '\# ')

ifeq ($(OS),Windows_NT)
    DETECTED_OS := Windows
    PROTOC_ARCHIVE := protoc-$(PROTOC_VERSION)-win64.zip
else
    UNAME := $(shell uname)
    ifeq ($(UNAME),Linux)
        DETECTED_OS := Linux
        PROTOC_ARCHIVE := protoc-$(PROTOC_VERSION)-linux-x86_64.zip
    else ifeq ($(UNAME),Darwin)
        DETECTED_OS := Darwin
        PROTOC_ARCHIVE := protoc-$(PROTOC_VERSION)-osx-universal_binary.zip
    endif
endif

PROTOC_DOWNLOAD_URL := https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/$(PROTOC_ARCHIVE)

proto-dependencies:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.31.0
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0
	@if [ -z "$(DETECTED_OS)" ]; then \
		echo "Error: Can't install protoc on your OS, please install protoc-$(PROTOC_VERSION) manually." >&2; \
		exit 1; \
	fi
	@echo "Downloading $(PROTOC_ARCHIVE)"
	curl --show-error --fail --no-location -LO $(PROTOC_DOWNLOAD_URL)
	unzip -o $(PROTOC_ARCHIVE) -d ./.protoc
	rm $(PROTOC_ARCHIVE)

generate-tools-installs: proto-dependencies
	go install github.com/mstoykov/enumer@v0.0.1 # TODO figure out if we shouldn't move to a different fork
	go install mvdan.cc/gofumpt@v0.8.0 # TODO maybe just use go fmt for this case

generate: generate-tools-installs
	PATH="$(PWD)/.protoc/bin:$(PATH)" go generate ./...

all: clean format tests build

## build: Builds the 'k6' binary.
build:
	go build

## format: Applies Go formatting to code.
format:
	find . -name '*.go' -exec gofmt -s -w {} +

## grpc-server-run: Runs the gRPC server example.
grpc-server-run:
	go run -mod=mod examples/grpc_server/*.go

$(LINT_DIR):
	mkdir -p $@

$(LINT_BASE): $(LINT_WORKFLOW) | $(LINT_DIR)
	curl -fsSL $(LINT_BASE_URL) -o $@

$(LINT_FINAL): $(LINT_BASE) $(wildcard $(LINT_PATCH))
	cp $(LINT_BASE) $@
	@if [ -f $(LINT_PATCH) ]; then \
	  echo "Applying $(LINT_PATCH)"; \
	  git apply --directory=$(LINT_DIR) $(LINT_PATCH); \
	fi

## lint: Run golangci-lint with the k6-ci config + $(LINT_PATCH).
lint: $(LINT_FINAL)
	echo "Running linters..."
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) \
	  run --config=$(LINT_FINAL) ./...

## update-lint-patch: Regenerate $(LINT_PATCH) from the locally edited $(LINT_FINAL).
update-lint-patch: $(LINT_BASE)
	@if [ ! -f $(LINT_FINAL) ]; then \
	  echo "Run 'make lint' first to materialize $(LINT_FINAL), edit it, then re-run."; \
	  exit 1; \
	fi
	-diff -u --label a/.golangci.yml --label b/.golangci.yml $(LINT_BASE) $(LINT_FINAL) > $(LINT_PATCH)

## clean-lint: Remove $(LINT_DIR).
clean-lint:
	rm -rf $(LINT_DIR)

## tests: Executes any unit tests.
tests:
	go test -race -timeout 210s ./...

## check: Runs the linters and tests.
check: lint tests

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
	@echo "cleaning"
	rm -f ./k6

.PHONY: build format lint tests check help update-lint-patch clean-lint
