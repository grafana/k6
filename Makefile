GOLANGCI_LINT_VERSION = $(shell head -n 1 .golangci.yml | tr -d '\# ')
TMPDIR ?= /tmp

all: build

build :
	go build

format :
	find . -name '*.go' -exec gofmt -s -w {} +

ci-like-lint :
	@docker run --rm -t -v $(shell pwd):/app \
		-v $(TMPDIR)/golangci-cache-$(GOLANGCI_LINT_VERSION):/golangci-cache \
		--env "GOLANGCI_LINT_CACHE=/golangci-cache" \
		-w /app golangci/golangci-lint:$(GOLANGCI_LINT_VERSION) \
		make lint

lint :
	golangci-lint run --out-format=tab --new-from-rev master ./...

tests :
	go test -race -timeout 210s ./...

check : ci-like-lint tests

.PHONY: build format ci-like-lint lint tests check
