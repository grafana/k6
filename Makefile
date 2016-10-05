all: build web docs

.PHONY: build
build: js/node_modules web
	go build

js/node_modules:
	cd js && npm install

.PHONY: web
web: web/node_modules web/bower_components
	cd web && ember build -prod

web/node_modules:
	cd web && npm install

web/bower_components:
	cd web && bower install

.PHONY: docs
docs:
	jsdoc -c jsdoc.json
