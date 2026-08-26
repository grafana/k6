# xk6-tcp Examples

This directory contains example scripts demonstrating various features of the xk6-tcp extension.

## Running Examples

All examples can be run using k6 with the xk6-tcp extension:

```bash
# Build k6 with xk6-tcp (if not already built)
make build

# Run examples with the built-in echo server wrapper
./with-echo ./k6 run examples/hello.js  # see "Building with-echo" below
```

The `with-echo` wrapper automatically:
- Starts TCP and HTTP echo servers on random localhost ports
- Sets `TCP_ECHO_HOST`, `TCP_ECHO_PORT`, `HTTP_ECHO_HOST`, `HTTP_ECHO_PORT`, and `HTTP_ECHO_URL` environment variables
- Runs your k6 script with these servers available
- Cleans up servers when the script completes

### Using External Servers

You can also run examples against custom TCP servers:

```bash
# With custom host/port
TCP_ECHO_HOST=example.com TCP_ECHO_PORT=9000 ./k6 run examples/hello.js
```

Or start a standalone echo server:

```bash
# Using netcat (Linux/Mac)
while true; do nc -l 8080 -c 'xargs -0 echo'; done

# Then run without with-echo wrapper
TCP_ECHO_HOST=localhost TCP_ECHO_PORT=8080 ./k6 run examples/hello.js
```

## Examples Overview

### [hello.js](hello.js)
**Async/await pattern**

Shows how to use async methods (`connect`, `write`) for cleaner code flow with promises.

**Key concepts:**
- Async/await operations
- Promise-based control flow
- Synchronous-style code
- Basic connect/write/read pattern

### [echo.js](echo.js)
**Bidirectional communication**

Demonstrates sending multiple messages and receiving echo responses in sequence.

**Key concepts:**
- Multiple write operations
- State management across callbacks
- Sequential message handling

### [timeout.js](timeout.js)
**Timeout handling**

Shows how to set and handle read timeouts for detecting idle connections.

**Key concepts:**
- `setTimeout()` method
- Timeout event handling
- Inactivity detection

### [options.js](options.js)
**Socket options and metrics tags**

Demonstrates using tags for organizing and filtering metrics in k6 output.

**Key concepts:**
- Socket constructor options
- Per-connection tags
- Per-operation tags
- Metrics organization

### [binary.js](binary.js)
**Binary protocol data**

Shows how to send and receive binary data using ArrayBuffer and Uint8Array.

**Key concepts:**
- ArrayBuffer usage
- Binary protocol headers
- Byte manipulation
- String.fromCharCode for binary-to-text conversion

### [multiple.js](multiple.js)
**Concurrent connections**

Demonstrates managing multiple TCP connections simultaneously with Promise coordination.

**Key concepts:**
- Multiple sockets
- Promise.all() coordination
- Concurrent operations
- Per-connection tagging

### [state.js](state.js)
**Socket state inspection**

Shows how to check socket properties like connection state and byte counters.

**Key concepts:**
- `ready_state` property
- `connected` property
- `bytes_written` counter
- `bytes_read` counter
- State lifecycle

### [smoke.test.js](smoke.test.js)
**Load testing example**

Demonstrates k6 load testing with TCP sockets, including multiple VUs, thresholds, and checks.

**Key concepts:**
- k6 options (vus, duration, thresholds)
- check() assertions
- Multiple concurrent VUs
- Performance validation
- Load testing patterns

### [tls.js](tls.js)
**TLS/SSL secure connections**

Shows how to establish an encrypted TCP connection, send an HTTPS-style request, and read back a response.

**Key concepts:**
- TLS encryption
- Secure connections
- HTTPS-like protocols
- Response handling over TLS
- k6 TLS configuration
- Certificate handling

### [tls_simple.js](tls_simple.js)
**Minimal TLS example**

Connects to `example.com:443` with `tls: true`, waits briefly, and then closes the socket.
This is the smallest possible TLS connection example in the repository.

**Key concepts:**
- Minimal TLS connection setup
- `connect()` with `tls: true`
- Short connect / close lifecycle

### [basic.js](basic.js)
**Minimal example**

The simplest possible socket creation example.

## Building with-echo

The `with-echo` binary is built from source in [../tools/with-echo](../tools/with-echo). To rebuild:

```bash
cd tools/with-echo
go build -o ../../with-echo
```

## Configuration

Examples use different environment variables depending on the script:

- `TCP_ECHO_HOST` - Target host (default: localhost, automatically set by with-echo)
- `TCP_ECHO_PORT` - Target port (default: 8080, automatically set by with-echo)
- `HTTP_ECHO_HOST` - HTTP echo server host (automatically set by with-echo)
- `HTTP_ECHO_PORT` - HTTP echo server port (automatically set by with-echo)
- `HTTP_ECHO_URL` - Full HTTP echo server URL (automatically set by with-echo)
- `TLS_HOST` - TLS target hostname for `tls.js` (default: `example.com`)
- `TLS_PORT` - TLS target port for `tls.js` (default: `443`)

Notes:

- `with-echo` sets the `TCP_ECHO_*` and `HTTP_ECHO_*` variables automatically.
- `tls.js` reads `TLS_HOST` and `TLS_PORT` if you want to target a different TLS endpoint.
- `tls_simple.js` does not use environment variables; it always connects to `example.com:443`.

## Testing with k6 Options

You can combine examples with k6 load testing features:

```bash
# Run with multiple VUs
./k6 run --vus 10 --duration 30s examples/hello.js

# With custom thresholds
./k6 run --vus 5 --duration 10s \
  --threshold 'tcp_socket_duration<100' \
  examples/hello.js
```

## Common Patterns

### Event Handler Pattern
```javascript
socket.on("event", (data) => {
  // Handle event
});
```

### Async Pattern
```javascript
await socket.connect(port, host);
await socket.write(data);
```

### Promise Coordination
```javascript
const promise = new Promise((resolve) => {
  socket.on("close", resolve);
});
await promise;
```

## Metrics

All examples generate k6 metrics:

- `tcp_socket_connecting` - Connection establishment time
- `tcp_socket_resolving` - DNS resolution time
- `tcp_socket_duration` - Total connection duration
- `tcp_sockets` - Number of sockets created
- `tcp_reads` - Number of read operations
- `tcp_writes` - Number of write operations
- `tcp_errors` - Number of errors
- `tcp_timeouts` - Number of timeouts
- `tcp_partial_writes` - Number of partial writes

## Troubleshooting

**Connection refused:**
- Make sure the TCP server is running
- Check firewall settings
- Verify host/port are correct

**Timeout:**
- Increase timeout value with `socket.setTimeout()`
- Check network connectivity
- Verify server is responding

**No data received:**
- Ensure server echoes data back
- Check write operation completed
- Verify data format matches server expectations

## See Also

- [Test files](../test/) - Additional examples used for testing
- [README](../README.md) - Main documentation
- [TypeScript definitions](../index.d.ts) - Complete API reference
