VERSION := 0.2.1

all: build web

.PHONY: build
build: web
	go build

.PHONY: check
check:
	gometalinter --deadline 10m --config gometalinter.json ./...
	go test -timeout 30s ./...

.PHONY: web
web: web/node_modules web/bower_components
	cd web && node node_modules/.bin/ember build -prod

web/node_modules:
	cd web && npm install

web/bower_components: web/node_modules
	cd web && node node_modules/.bin/bower install --allow-root

.PHONY: docs
docs:
	jsdoc -c jsdoc.json

.PHONY: container
container:
	docker build --rm --pull --no-cache -t loadimpact/k6:$(VERSION) .

.PHONY: push
push:
	docker push loadimpact/k6:$(VERSION)
