MAKEFLAGS += --silent
GOLANGCI_LINT_VERSION = $(shell head -n 1 .golangci.yml | tr -d '\# ')

all: clean format tests build

## build: Builds the 'k6' binary.
build:
	go build

## format: Applies Go formatting to code.
format:
	find . -name '*.go' -exec gofmt -s -w {} +

## check-linter-version: Checks if the linter version is the same as the one specified in the linter config.
check-linter-version:
	(golangci-lint version | grep "version $(shell head -n 1 .golangci.yml | tr -d '\# ')") || echo "Your installation of golangci-lint is different from the one that is specified in k6's linter config (there it's $(shell head -n 1 .golangci.yml | tr -d '\# ')). Results could be different in the CI."

## lint: Runs the linters.
lint: check-linter-version
	echo "Running linters..."
	golangci-lint run --out-format=tab --new-from-rev master ./...

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
