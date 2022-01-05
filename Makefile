GOLANGCI_LINT_VERSION = $(shell head -n 1 .golangci.yml | tr -d '# ')

all: build

build:
	go build

format:
	find . -name '*.go' -exec gofmt -s -w {} +

ci-like-lint :
	docker run --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:$(GOLANGCI_LINT_VERSION) make lint

lint :
	golangci-lint run --out-format=tab --new-from-rev master ./...

tests :
	go test -race -timeout 210s ./...

check : lint tests

container:
	docker build --rm --pull --no-cache -t loadimpact/k6 .

.PHONY: build format ci-like-lint lint tests check container
