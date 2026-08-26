# xk6-tcp

**TCP protocol support for k6**

**xk6-tcp** is a k6 extension that adds first-class support for raw TCP socket communication to your load testing and performance scripts. With this extension, you can establish TCP connections, send and receive data, and test network protocols directly from your k6 tests.

The API is intentionally designed to feel familiar to users of Node.js's [`net.Socket`](https://nodejs.org/api/net.html#class-netsocket) API, with event-driven programming, Promise-based operations, and comprehensive lifecycle management. This provides a modern, ergonomic developer experience for TCP-based protocol testing in JavaScript.

## Example

The basic structure of a TCP test uses event-driven programming. You create a socket, register event handlers, and connect to the remote server. The socket remains active until explicitly closed.

```javascript file=script.js
import tcp from "k6/x/tcp"

export default async function () {
  const socket = new tcp.Socket()
  const closePromise = new Promise((resolve) => {
    socket.on("close", () => {
      console.log("Connection closed")
      resolve()
    })
  })

  socket.on("data", (data) => {
    console.log("Received data")
    const str = String.fromCharCode.apply(null, new Uint8Array(data))
    console.log(str)
    socket.destroy()
  })

  socket.on("error", (err) => {
    console.log(`Socket error: ${err}`)
  })

  await socket.connect(__ENV.TCP_ECHO_PORT, __ENV.TCP_ECHO_HOST)
  console.log("Connected")
  await socket.write("Hey there\n")

  await closePromise
}
```

## Examples

The [examples](./examples/) directory contains comprehensive examples demonstrating various features. See the [examples README](./examples/README.md) for detailed documentation and usage instructions. Additional test examples can be found in the [test](./test/) directory.

## Async Programming

**xk6-tcp** uses an async-first JavaScript API. Methods like `connect()` and `write()` return Promises, so they work naturally with `async`/`await`, while socket lifecycle and data events continue to flow through `.on()` handlers. You can also combine socket operations with standard JavaScript asynchronous constructs like [setTimeout()](https://developer.mozilla.org/en-US/docs/Web/API/Window/setTimeout) and [setInterval()](https://developer.mozilla.org/en-US/docs/Web/API/Window/setInterval).

## TLS/SSL Support

**xk6-tcp** supports secure TCP connections using TLS/SSL encryption. Enable TLS by setting the `tls` option when connecting:

```javascript
await socket.connect({
  port: 443,
  host: "secure.example.com",
  tls: true  // Enable TLS encryption
});
```

TLS configuration (certificates, verification, cipher suites, etc.) is handled by k6's standard [TLS configuration](https://grafana.com/docs/k6/latest/using-k6/protocols/ssl-tls/). This ensures consistency across your k6 tests and leverages k6's robust TLS support.

**Common Use Cases:**
- Secure database connections (PostgreSQL, MySQL, Redis with TLS)
- HTTPS-like protocols over raw TCP
- SMTPS, IMAPS, and other secure mail protocols
- Message queues with TLS (Kafka, RabbitMQ)
- Custom secure protocols

See [examples/tls.js](./examples/tls.js) and [examples/tls_simple.js](./examples/tls_simple.js) for complete examples.

## Event-Driven Usage

Register event handlers for connection lifecycle and data events using the `.on()` method:

| Event       | Description
|-------------|------------------------------------------------------------------
| `connect`   | Triggered when the socket successfully establishes a connection to the remote server
| `data`      | Triggered when data is received from the remote endpoint
| `close`     | Triggered when the socket connection is fully closed
| `error`     | Triggered when a socket error occurs (connection failures, network issues, etc.)
| `timeout`   | Triggered when the socket times out due to inactivity (see `setTimeout()`)

All event handlers are executed in the context of the k6 VU event loop.

## API Overview

### Socket Constructor

```javascript
const socket = new Socket(options);
```

**Options:**
- `tags` (optional): Key-value pairs for metrics collection

### Methods

| Method | Description
|--------|-------------
| `connect(port, host?)` | Returns a Promise that resolves when the connection is established
| `connect(options)` | Returns a Promise that resolves when the connection is established with options
| `write(data, options?)` | Returns a Promise that resolves when the data has been written
| `setTimeout(timeout)` | Sets inactivity timeout in milliseconds (0 to disable)
| `destroy()` | Destroys the socket and closes the connection
| `on(event, handler)` | Registers an event handler

**Write Options:**
- `encoding` (optional): For string data, supports `utf8`, `utf-8`, `ascii`, `base64`, `base64url`, and `hex`; unsupported values are rejected. Ignored for `ArrayBuffer` input.
- `tags` (optional): Key-value pairs for write-specific metrics

### Properties

| Property | Type | Description
|----------|------|-------------
| `ready_state` | string | Current socket state: `'disconnected'`, `'opening'`, `'open'`, or `'destroyed'`
| `connected` | boolean | `true` if socket is connected (ready_state is 'open')
| `local_ip` | string | Local IP address
| `local_port` | number | Local port number
| `remote_ip` | string | Remote IP address
| `remote_port` | number | Remote port number
| `bytes_written` | number | Total bytes sent
| `bytes_read` | number | Total bytes received

## Metrics

The table below lists all the metrics generated by **xk6-tcp** during socket operations:

| Metric Name               | Type    | Description
|---------------------------|---------|--------------------------------------------------------
| `tcp_socket_connecting`   | Trend   | Time taken to establish TCP connection (milliseconds)
| `tcp_socket_resolving`    | Trend   | Time taken to resolve hostname (milliseconds)
| `tcp_socket_duration`     | Trend   | Total duration of socket connection (milliseconds)
| `tcp_sockets`             | Counter | Number of TCP socket connections established
| `tcp_reads`               | Counter | Number of read operations
| `tcp_writes`              | Counter | Number of write operations
| `tcp_errors`              | Counter | Number of TCP errors
| `tcp_timeouts`            | Counter | Number of socket timeouts
| `tcp_partial_writes`      | Counter | Number of partial write failures (when only some data was written before error)
| `data_sent`               | Counter | Total bytes sent (builtin k6 metric)
| `data_received`           | Counter | Total bytes received (builtin k6 metric)

You can pass custom `tags` in the Socket constructor, connection options, or write options to include additional metadata with each metric.

## Download

> [!NOTE]
> With k6's [Automatic Extension Resolution](https://grafana.com/docs/k6/latest/extensions/guides/what-are-k6-extensions/#automatic-extension-resolution), you don't need to manually build or download a custom k6 binary. Simply import the extension in your script using `import { Socket } from "k6/x/tcp"`, and k6 will automatically download and build it for you.

You can download pre-built k6 binaries from the [Releases](https://github.com/grafana/xk6-tcp/releases/) page.

**Build**

The [xk6](https://github.com/grafana/xk6) build tool can be used to build a k6 binary that includes the xk6-tcp extension:

```bash
$ xk6 build --with github.com/grafana/xk6-tcp@latest
```

For more build options and how to use xk6, check out the [xk6 documentation](https://github.com/grafana/xk6).

## Documentation

Generated API documentation is available at [grafana.github.io/xk6-tcp](https://grafana.github.io/xk6-tcp)

## Contribute

If you wish to contribute to this project, please start by reading the [Contributing Guidelines](CONTRIBUTING.md).
