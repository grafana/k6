GOLANGCI_LINT_VERSION = $(shell head -n 1 .golangci.yml | tr -d '\# ')
TMPDIR ?= /tmp

test:
	go test -race -timeout 800s ./...

ci-like-lint:
	@docker run --rm -t -v $(shell pwd):/app \
		-v $(TMPDIR)/golangci-cache-$(GOLANGCI_LINT_VERSION):/golangci-cache \
		--env "GOLANGCI_LINT_CACHE=/golangci-cache" \
		-w /app golangci/golangci-lint:$(GOLANGCI_LINT_VERSION) \
		make lint

lint:
	golangci-lint run --out-format=tab ./...

check: ci-like-lint test

.PHONY: test lint ci-like-lint check