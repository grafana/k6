GOLANGCI_LINT_VERSION = $(shell head -n 1 .golangci.yml | tr -d '\# ')
TMPDIR ?= /tmp

build: ## Build the binary
	xk6 build --with github.com/grafana/xk6-websockets=. --with github.com/grafana/xk6-timers

ws-echo-server-run: ## Run the ws-echo-server
	docker run --detach --rm --name ws-echo-server -p 10000:8080 jmalloc/echo-server

ws-echo-server-stop: ## Stop the ws-echo-server
	docker stop ws-echo-server

test:
	go test -race -timeout 30s ./...

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