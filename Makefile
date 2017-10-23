VERSION := 0.2.2

all: build

.PHONY: build
build:
	go build

.PHONY: check
check:
	gometalinter --deadline 10m --config gometalinter.json ./...
	go test -timeout 30s ./...

.PHONY: docs
docs:
	jsdoc -c jsdoc.json

.PHONY: container
container:
	docker build --rm --pull --no-cache -t loadimpact/k6:$(VERSION) .

.PHONY: push
push:
	docker push loadimpact/k6:$(VERSION)
