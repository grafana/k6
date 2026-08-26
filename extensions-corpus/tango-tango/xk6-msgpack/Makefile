.PHONY: build clean k6

build: k6
	xk6 build --with github.com/tangotango/xk6-msgpack=.

k6:
	@which xk6 > /dev/null || go install go.k6.io/xk6/cmd/xk6@latest

clean:
	rm -f k6 go.sum

test:
	go test -v ./...
