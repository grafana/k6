VERSION := 0.2.2

all: build

.PHONY: build
build:
	go build

.PHONY: format
format:
	find . -name '*.go' -exec gofmt -s -w {} +

.PHONY: plugin
plugin:
	pushd /tmp; go build -buildmode=plugin -o /tmp/leftpad.so github.com/andremedeiros/leftpad; popd

.PHONY: check
check: plugin
	golangci-lint run --out-format=tab --new-from-rev master ./...
	go test -race -timeout 210s ./...

.PHONY: docs
docs:
	jsdoc -c jsdoc.json

.PHONY: container
container:
	docker build --rm --pull --no-cache -t loadimpact/k6:$(VERSION) .

.PHONY: push
push:
	docker push loadimpact/k6:$(VERSION)
