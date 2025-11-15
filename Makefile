MAKEFLAGS += --silent
GOLANGCI_LINT_VERSION = $(shell head -n 1 .golangci.yml | tr -d '\# ')
PROTOC_VERSION := 21.12

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
	go install github.com/mailru/easyjson/easyjson@v0.7.7 # TODO remove this in the future

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

## check-linter-version: Checks if the linter version is the same as the one specified in the linter config.
check-linter-version:
	(golangci-lint version | grep -E "version v?$(shell head -n 1 .golangci.yml | tr -d '\# v')") || echo "Your installation of golangci-lint is different from the one that is specified in k6's linter config (there it's $(shell head -n 1 .golangci.yml | tr -d '\# ')). Results could be different in the CI."

## lint: Runs the linters.
lint: check-linter-version
	echo "Running linters..."
	golangci-lint run ./...

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

.PHONY: build format lint tests check check-linter-version help
