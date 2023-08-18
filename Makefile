MAKEFLAGS += --silent
GOLANGCI_CONFIG ?= .golangci.yml

all: clean format test build

## help: Prints a list of available build targets.
help:
	echo "Usage: make <OPTIONS> ... <TARGETS>"
	echo ""
	echo "Available targets are:"
	echo ''
	sed -n 's/^##//p' ${PWD}/Makefile | column -t -s ':' | sed -e 's/^/ /'
	echo
	echo "Targets run by default are: `sed -n 's/^all: //p' ./Makefile | sed -e 's/ /, /g' | sed -e 's/\(.*\), /\1, and /'`"

## clean: Removes any previously created artifacts.
clean:
	rm -f ./k6 .golangci.yml	

## build: Builds a custom 'k6' with the local extension. 
build:
	go install go.k6.io/xk6/cmd/xk6@latest
	xk6 build --with github.com/grafana/xk6-output-prometheus-remote=.

## format: Applies Go formatting to code.
format:
	go fmt ./...

## linter-config: Checks if the linter config exists, if not, downloads it from the main k6 repository.
linter-config:
	test -s "${GOLANGCI_CONFIG}" || (echo "No linter config, downloading from main k6 repository..." && curl --silent --show-error --fail --no-location https://raw.githubusercontent.com/grafana/k6/master/.golangci.yml --output "${GOLANGCI_CONFIG}")

## check-linter-version: Checks if the linter version is the same as the one specified in the linter config.
check-linter-version:
	(golangci-lint version | grep "version $(shell head -n 1 .golangci.yml | tr -d '\# ')") || echo "Your installation of golangci-lint is different from the one that is specified in k6's linter config (there it's $(shell head -n 1 .golangci.yml | tr -d '\# ')). Results could be different in the CI."

## lint: Runs the linters.
lint: linter-config check-linter-version
	echo "Running linters..."
	golangci-lint run --out-format=tab ./...

## check: Runs the linters and tests.
check: lint test

## test: Executes any unit tests.
test:
	go test -cover -race ./...

.PHONY: build clean format help test lint check check-linter-version linter-config
