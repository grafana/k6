# xk6-websockets

This extension adds a PoC [Websockets API](https://websockets.spec.whatwg.org) implementation to [k6](https://www.k6.io).

This is meant to try to implement the specification as close as possible without doing stuff that don't make sense in k6 like:
1. not reporting errors
2. not allowing some ports and other security workarounds
3. supporting Blob as message

It likely in the future will support additional k6 specific features such as:
1. adding additional tags
2. support for ping/pong which isn't part of the specification

It is implemented using the [xk6](https://k6.io/blog/extending-k6-with-xk6/) system.

## Getting started  

1. Install `xk6`:
  ```shell
  $ go install go.k6.io/xk6/cmd/xk6@latest
  ```

2. Build the binary:
  ```shell
  $ xk6 build --with github.com/grafana/xk6-websockets
  ```
