GOLANGCI_LINT_VERSION = $(shell head -n 1 .golangci.yml | tr -d '\# ')
TMPDIR ?= /tmp
BASEREV = $(shell git merge-base HEAD origin/main)

all: build

build :
	go install go.k6.io/xk6/cmd/xk6@latest && xk6 build --output xk6-browser --with github.com/grafana/xk6-browser=.

format :
	find . -name '*.go' -exec gofmt -s -w {} +

ci-like-lint :
	@docker run --rm -t -v $(shell pwd):/app \
		-v $(TMPDIR)/golangci-cache-$(GOLANGCI_LINT_VERSION):/golangci-cache \
		--env "GOLANGCI_LINT_CACHE=/golangci-cache" \
		-w /app golangci/golangci-lint:$(GOLANGCI_LINT_VERSION) \
		make lint

lint :
	golangci-lint run --timeout=3m --out-format=tab --new-from-rev "$(BASEREV)" ./...

tests :
	go test -race -timeout 210s ./...

check : ci-like-lint tests

container:
	docker build --rm --pull --no-cache -t grafana/xk6-browser .

.PHONY: build format ci-like-lint lint tests check container
