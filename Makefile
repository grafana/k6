all: build

.PHONY: build
build:
	go build

.PHONY: format
format:
	find . -name '*.go' -exec gofmt -s -w {} +

.PHONY: ci-like-lint
ci-like-lint :
	docker run --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:$(shell head -n 1 .golangci.version) make lint

.PHONY: lint
lint :
	golangci-lint run --out-format=tab --new-from-rev master ./...

.PHONY: tests
tests :
	go test -race -timeout 210s ./...

.PHONY: check
check : lint tests

.PHONY: container
container:
	docker build --rm --pull --no-cache -t loadimpact/k6 .
