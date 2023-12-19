GO_MOD_DIRS := $(shell find . -type f -name 'go.mod' -exec dirname {} \; | sort)

test: testdeps
	set -e; for dir in $(GO_MOD_DIRS); do \
	  echo "go test in $${dir}"; \
	  (cd "$${dir}" && \
	    go mod tidy -compat=1.18 && \
	    go test && \
	    go test ./... -short -race && \
	    go test ./... -run=NONE -bench=. -benchmem && \
	    env GOOS=linux GOARCH=386 go test && \
	    go vet); \
	done
	cd internal/customvet && go build .
	go vet -vettool ./internal/customvet/customvet

testdeps: testdata/redis/src/redis-server

bench: testdeps
	go test ./... -test.run=NONE -test.bench=. -test.benchmem

.PHONY: all test testdeps bench

testdata/redis:
	mkdir -p $@
	wget -qO- https://download.redis.io/releases/redis-7.2-rc1.tar.gz | tar xvz --strip-components=1 -C $@

testdata/redis/src/redis-server: testdata/redis
	cd $< && make all

fmt:
	gofmt -w -s ./
	goimports -w  -local github.com/redis/go-redis ./

go_mod_tidy:
	set -e; for dir in $(GO_MOD_DIRS); do \
	  echo "go mod tidy in $${dir}"; \
	  (cd "$${dir}" && \
	    go get -u ./... && \
	    go mod tidy -compat=1.18); \
	done
