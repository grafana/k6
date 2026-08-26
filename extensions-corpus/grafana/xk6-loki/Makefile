PWD := $(shell pwd)
GO_FILES := $(shell find . -type f -name "*.go" -print)

.PHONY: run

k6: $(GO_FILES)
	xk6 build master \
		--replace "github.com/gocql/gocql=github.com/grafana/gocql@v0.0.0-20200605141915-ba5dc39ece85" \
    --with "github.com/grafana/xk6-loki=$(PWD)"

go.sum: $(GO_FILES) go.mod
	go mod tidy

run: k6
	$(PWD)/k6 run examples/simple.js
