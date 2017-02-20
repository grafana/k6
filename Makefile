VERSION := 0.2.1

all: build web

.PHONY: build
build: web
	go build

.PHONY: web
web: web/node_modules web/bower_components
	cd web && ember build -prod

web/node_modules:
	cd web && npm install

web/bower_components:
	cd web && bower install --allow-root

.PHONY: docs
docs:
	jsdoc -c jsdoc.json

.PHONY: container
container:
	docker build --rm --pull --no-cache -t loadimpact/k6:$(VERSION) .

.PHONY: push
push:
	docker push loadimpact/k6:$(VERSION)
