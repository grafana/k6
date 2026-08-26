all: test build example

test:

build: k6

k6: *.go go.mod go.sum
	xk6 build --with github.com/grafana/xk6-sql@latest --with github.com/grafana/xk6-sql-driver-azuresql=.

example: k6
	./k6 run examples/example.js

.PHONY: test all example
