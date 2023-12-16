# A QUIC implementation in pure Go

<img src="docs/quic.png" width=303 height=124>

[![PkgGoDev](https://pkg.go.dev/badge/github.com/quic-go/quic-go)](https://pkg.go.dev/github.com/quic-go/quic-go)
[![Code Coverage](https://img.shields.io/codecov/c/github/quic-go/quic-go/master.svg?style=flat-square)](https://codecov.io/gh/quic-go/quic-go/)
[![Fuzzing Status](https://oss-fuzz-build-logs.storage.googleapis.com/badges/quic-go.svg)](https://bugs.chromium.org/p/oss-fuzz/issues/list?sort=-opened&can=1&q=proj:quic-go)

quic-go is an implementation of the QUIC protocol ([RFC 9000](https://datatracker.ietf.org/doc/html/rfc9000), [RFC 9001](https://datatracker.ietf.org/doc/html/rfc9001), [RFC 9002](https://datatracker.ietf.org/doc/html/rfc9002)) in Go. It has support for HTTP/3 ([RFC 9114](https://datatracker.ietf.org/doc/html/rfc9114)), including QPACK ([RFC 9204](https://datatracker.ietf.org/doc/html/rfc9204)).

In addition to these base RFCs, it also implements the following RFCs: 
* Unreliable Datagram Extension ([RFC 9221](https://datatracker.ietf.org/doc/html/rfc9221))
* Datagram Packetization Layer Path MTU Discovery (DPLPMTUD, [RFC 8899](https://datatracker.ietf.org/doc/html/rfc8899))
* QUIC Version 2 ([RFC 9369](https://datatracker.ietf.org/doc/html/rfc9369))
* QUIC Event Logging using qlog ([draft-ietf-quic-qlog-main-schema](https://datatracker.ietf.org/doc/draft-ietf-quic-qlog-main-schema/) and [draft-ietf-quic-qlog-quic-events](https://datatracker.ietf.org/doc/draft-ietf-quic-qlog-quic-events/))

Support for WebTransport over HTTP/3 ([draft-ietf-webtrans-http3](https://datatracker.ietf.org/doc/draft-ietf-webtrans-http3/)) is implemented in [webtransport-go](https://github.com/quic-go/webtransport-go).

## Using QUIC

### Running a Server

The central entry point is the `quic.Transport`. A transport manages QUIC connections running on a single UDP socket. Since QUIC uses Connection IDs, it can demultiplex a listener (accepting incoming connections) and an arbitrary number of outgoing QUIC connections on the same UDP socket.

```go
udpConn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: 1234})
// ... error handling
tr := quic.Transport{
  Conn: udpConn,
}
ln, err := tr.Listen(tlsConf, quicConf)
// ... error handling
go func() {
  for {
    conn, err := ln.Accept()
    // ... error handling
    // handle the connection, usually in a new Go routine
  }
}()
```

The listener `ln` can now be used to accept incoming QUIC connections by (repeatedly) calling the `Accept` method (see below for more information on the `quic.Connection`).

As a shortcut,  `quic.Listen` and `quic.ListenAddr` can be used without explicitly initializing a `quic.Transport`:

```
ln, err := quic.Listen(udpConn, tlsConf, quicConf)
```

When using the shortcut, it's not possible to reuse the same UDP socket for outgoing connections.

### Running a Client

As mentioned above, multiple outgoing connections can share a single UDP socket, since QUIC uses Connection IDs to demultiplex connections.

```go
ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second) // 3s handshake timeout
defer cancel()
conn, err := tr.Dial(ctx, <server address>, <tls.Config>, <quic.Config>)
// ... error handling
```

As a shortcut, `quic.Dial` and `quic.DialAddr` can be used without explictly initializing a `quic.Transport`:

```go
ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second) // 3s handshake timeout
defer cancel()
conn, err := quic.Dial(ctx, conn, <server address>, <tls.Config>, <quic.Config>)
```

Just as we saw before when used a similar shortcut to run a server, it's also not possible to reuse the same UDP socket for other outgoing connections, or to listen for incoming connections.

### Using a QUIC Connection

#### Accepting Streams

QUIC is a stream-multiplexed transport. A `quic.Connection` fundamentally differs from the `net.Conn` and the `net.PacketConn` interface defined in the standard library. Data is sent and received on (unidirectional and bidirectional) streams (and, if supported, in [datagrams](#quic-datagrams)), not on the connection itself. The stream state machine is described in detail in [Section 3 of RFC 9000](https://datatracker.ietf.org/doc/html/rfc9000#section-3).

Note: A unidirectional stream is a stream that the initiator can only write to (`quic.SendStream`), and the receiver can only read from (`quic.ReceiveStream`). A bidirectional stream (`quic.Stream`) allows reading from and writing to for both sides.

On the receiver side, streams are accepted using the `AcceptStream` (for bidirectional) and `AcceptUniStream` functions. For most user cases, it makes sense to call these functions in a loop:

```go
for {
  str, err := conn.AcceptStream(context.Background()) // for bidirectional streams
  // ... error handling
  // handle the stream, usually in a new Go routine
}
```

These functions return an error when the underlying QUIC connection is closed.

#### Opening Streams

There are two slightly different ways to open streams, one synchronous and one (potentially) asynchronous. This API is necessary since the receiver grants us a certain number of streams that we're allowed to open. It may grant us additional streams later on (typically when existing streams are closed), but it means that at the time we want to open a new stream, we might not be able to do so.

Using the synchronous method `OpenStreamSync` for bidirectional streams, and `OpenUniStreamSync` for unidirectional streams, an application can block until the peer allows opening additional streams. In case that we're allowed to open a new stream, these methods return right away:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
str, err := conn.OpenStreamSync(ctx) // wait up to 5s to open a new bidirectional stream
```

The asynchronous version never blocks. If it's currently not possible to open a new stream, it returns a `net.Error` timeout error:

```go
str, err := conn.OpenStream()
if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
  // It's currently not possible to open another stream,
  // but it might be possible later, once the peer allowed us to do so.
}
```

These functions return an error when the underlying QUIC connection is closed.

#### Using Streams

Using QUIC streams is pretty straightforward. The `quic.ReceiveStream` implements the `io.Reader` interface, and the `quic.SendStream` implements the `io.Writer` interface. A bidirectional stream (`quic.Stream`) implements both these interfaces. Conceptually, a bidirectional stream can be thought of as the composition of two unidirectional streams in opposite directions.

Calling `Close` on a `quic.SendStream` or a `quic.Stream` closes the send side of the stream. On the receiver side, this will be surfaced as an `io.EOF` returned from the `io.Reader` once all data has been consumed. Note that for bidirectional streams, `Close` _only_ closes the send side of the stream. It is still possible to read from the stream until the peer closes or resets the stream.

In case the application wishes to abort sending on a `quic.SendStream` or a `quic.Stream` , it can reset the send side by calling `CancelWrite` with an application-defined error code (an unsigned 62-bit number). On the receiver side, this surfaced as a `quic.StreamError` containing that error code on the `io.Reader`. Note that for bidirectional streams, `CancelWrite` _only_ resets the send side of the stream. It is still possible to read from the stream until the peer closes or resets the stream.

Conversely, in case the application wishes to abort receiving from a `quic.ReceiveStream` or a `quic.Stream`, it can ask the sender to abort data transmission by calling `CancelRead` with an application-defined error code (an unsigned 62-bit number). On the receiver side, this surfaced as a `quic.StreamError` containing that error code on the `io.Writer`. Note that for bidirectional streams, `CancelWrite` _only_ resets the receive side of the stream. It is still possible to write to the stream.

A bidirectional stream is only closed once both the read and the write side of the stream have been either closed and reset. Only then the peer is granted a new stream according to the maximum number of concurrent streams configured via `quic.Config.MaxIncomingStreams`.

### Configuring QUIC

The `quic.Config` struct passed to both the listen and dial calls (see above) contains a wide range of configuration options for QUIC connections, incl. the ability to fine-tune flow control limits, the number of streams that the peer is allowed to open concurrently, keep-alives, idle timeouts, and many more. Please refer to the documentation for the `quic.Config` for details.

The `quic.Transport` contains a few configuration options that don't apply to any single QUIC connection, but to all connections handled by that transport. It is highly recommend to set the `StatelessResetToken`, which allows endpoints to quickly recover from crashes / reboots of our node (see [Section 10.3 of RFC 9000](https://datatracker.ietf.org/doc/html/rfc9000#section-10.3)).

### Closing a Connection

#### When the remote Peer closes the Connection

In case the peer closes the QUIC connection, all calls to open streams, accept streams, as well as all methods on streams immediately return an error. Additionally, it is set as cancellation cause of the connection context. Users can use errors assertions to find out what exactly went wrong:

* `quic.VersionNegotiationError`: Happens during the handshake, if there is no overlap between our and the remote's supported QUIC versions.
* `quic.HandshakeTimeoutError`: Happens if the QUIC handshake doesn't complete within the time specified in `quic.Config.HandshakeTimeout`.
* `quic.IdleTimeoutError`: Happens after completion of the handshake if the connection is idle for longer than the minimum of both peers idle timeouts (as configured by `quic.Config.IdleTimeout`). The connection is considered idle when no stream data (and datagrams, if applicable) are exchanged for that period. The QUIC connection can be instructed to regularly send a packet to prevent a connection from going idle by setting `quic.Config.KeepAlive`. However, this is no guarantee that the peer doesn't suddenly go away (e.g. by abruptly shutting down the node or by crashing), or by a NAT binding expiring, in which case this error might still occur.
* `quic.StatelessResetError`: Happens when the remote peer lost the state required to decrypt the packet. This requires the `quic.Transport.StatelessResetToken` to be configured by the peer.
* `quic.TransportError`: Happens if when the QUIC protocol is violated. Unless the error code is `APPLICATION_ERROR`, this will not happen unless one of the QUIC stacks involved is misbehaving. Please open an issue if you encounter this error.
* `quic.ApplicationError`: Happens when the remote decides to close the connection, see below.

#### Initiated by the Application

A `quic.Connection` can be closed using `CloseWithError`:

```go
conn.CloseWithError(0x42, "error 0x42 occurred")
```

Applications can transmit both an error code (an unsigned 62-bit number) as well as a UTF-8 encoded human-readable reason. The error code allows the receiver to learn why the connection was closed, and the reason can be useful for debugging purposes.

On the receiver side, this is surfaced as a `quic.ApplicationError`.

### QUIC Datagrams

Unreliable datagrams are a QUIC extension ([RFC 9221](https://datatracker.ietf.org/doc/html/rfc9221)) that is negotiated during the handshake. Support can be enabled by setting the `quic.Config.EnableDatagram` flag. Note that this doesn't guarantee that the peer also supports datagrams. Whether or not the feature negotiation succeeded can be learned from the `quic.ConnectionState.SupportsDatagrams` obtained from `quic.Connection.ConnectionState()`.

QUIC DATAGRAMs are a new QUIC frame type sent in QUIC 1-RTT packets (i.e. after completion of the handshake). Therefore, they're end-to-end encrypted and congestion-controlled. However, if a DATAGRAM frame is deemed lost by QUIC's loss detection mechanism, they are not retransmitted.

Datagrams are sent using the `SendDatagram` method on the `quic.Connection`:

```go
conn.SendDatagram([]byte("foobar"))
```

And received using `ReceiveDatagram`:

```go
msg, err := conn.ReceiveDatagram()
```

Note that this code path is currently not optimized. It works for datagrams that are sent occasionally, but it doesn't achieve the same throughput as writing data on a stream. Please get in touch on issue #3766 if your use case relies on high datagram throughput, or if you'd like to help fix this issue. There are also some restrictions regarding the maximum message size (see #3599).

### QUIC Event Logging using qlog

quic-go logs a wide range of events defined in [draft-ietf-quic-qlog-quic-events](https://datatracker.ietf.org/doc/draft-ietf-quic-qlog-quic-events/), providing comprehensive insights in the internals of a QUIC connection. 

qlog files can be processed by a number of 3rd-party tools. [qviz](https://qvis.quictools.info/) has proven very useful for debugging all kinds of QUIC connection failures.

qlog is activated by setting a `Tracer` callback on the `Config`. It is called as soon as quic-go decides to starts the QUIC handshake on a new connection.
A useful implementation of this callback could look like this:
```go
quic.Config{
  Tracer: func(ctx context.Context, p logging.Perspective, connID quic.ConnectionID) *logging.ConnectionTracer {
    role := "server"
    if p == logging.PerspectiveClient {
      role = "client"
    }
    filename := fmt.Sprintf("./log_%x_%s.qlog", connID, role)
    f, err := os.Create(filename)
    // handle the error
    return qlog.NewConnectionTracer(f, p, connID)
  }
}
```

This implementation of the callback creates a new qlog file in the current directory named `log_<client / server>_<QUIC connection ID>.qlog`.


## Using HTTP/3

### As a server

See the [example server](example/main.go). Starting a QUIC server is very similar to the standard library http package in Go:

```go
http.Handle("/", http.FileServer(http.Dir(wwwDir)))
http3.ListenAndServeQUIC("localhost:4242", "/path/to/cert/chain.pem", "/path/to/privkey.pem", nil)
```

### As a client

See the [example client](example/client/main.go). Use a `http3.RoundTripper` as a `Transport` in a `http.Client`.

```go
http.Client{
  Transport: &http3.RoundTripper{},
}
```

## Projects using quic-go

| Project                                                   | Description                                                                                                                                                       | Stars                                                                                               |
| --------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| [AdGuardHome](https://github.com/AdguardTeam/AdGuardHome) | Free and open source, powerful network-wide ads & trackers blocking DNS server.                                                                                   | ![GitHub Repo stars](https://img.shields.io/github/stars/AdguardTeam/AdGuardHome?style=flat-square) |
| [algernon](https://github.com/xyproto/algernon)           | Small self-contained pure-Go web server with Lua, Markdown, HTTP/2, QUIC, Redis and PostgreSQL support                                                            | ![GitHub Repo stars](https://img.shields.io/github/stars/xyproto/algernon?style=flat-square)        |
| [caddy](https://github.com/caddyserver/caddy/)            | Fast, multi-platform web server with automatic HTTPS                                                                                                              | ![GitHub Repo stars](https://img.shields.io/github/stars/caddyserver/caddy?style=flat-square)       |
| [cloudflared](https://github.com/cloudflare/cloudflared)  | A tunneling daemon that proxies traffic from the Cloudflare network to your origins                                                                               | ![GitHub Repo stars](https://img.shields.io/github/stars/cloudflare/cloudflared?style=flat-square)  |
| [go-libp2p](https://github.com/libp2p/go-libp2p)          | libp2p implementation in Go, powering [Kubo](https://github.com/ipfs/kubo) (IPFS) and [Lotus](https://github.com/filecoin-project/lotus) (Filecoin), among others | ![GitHub Repo stars](https://img.shields.io/github/stars/libp2p/go-libp2p?style=flat-square)        |
| [Hysteria](https://github.com/apernet/hysteria)           | A powerful, lightning fast and censorship resistant proxy                                                                                                         | ![GitHub Repo stars](https://img.shields.io/github/stars/apernet/hysteria?style=flat-square)        |
| [Mercure](https://github.com/dunglas/mercure)             | An open, easy, fast, reliable and battery-efficient solution for real-time communications                                                                         | ![GitHub Repo stars](https://img.shields.io/github/stars/dunglas/mercure?style=flat-square)         |
| [OONI Probe](https://github.com/ooni/probe-cli)           | Next generation OONI Probe. Library and CLI tool.                                                                                                                 | ![GitHub Repo stars](https://img.shields.io/github/stars/ooni/probe-cli?style=flat-square)          |
| [syncthing](https://github.com/syncthing/syncthing/)      | Open Source Continuous File Synchronization                                                                                                                       | ![GitHub Repo stars](https://img.shields.io/github/stars/syncthing/syncthing?style=flat-square)     |
| [traefik](https://github.com/traefik/traefik)             | The Cloud Native Application Proxy                                                                                                                                | ![GitHub Repo stars](https://img.shields.io/github/stars/traefik/traefik?style=flat-square)         |
| [v2ray-core](https://github.com/v2fly/v2ray-core)         | A platform for building proxies to bypass network restrictions                                                                                                    | ![GitHub Repo stars](https://img.shields.io/github/stars/v2fly/v2ray-core?style=flat-square)        |
| [YoMo](https://github.com/yomorun/yomo)                   | Streaming Serverless Framework for Geo-distributed System                                                                                                         | ![GitHub Repo stars](https://img.shields.io/github/stars/yomorun/yomo?style=flat-square)            |

If you'd like to see your project added to this list, please send us a PR.

## Release Policy

quic-go always aims to support the latest two Go releases.

### Dependency on forked crypto/tls

Since the standard library didn't provide any QUIC APIs before the Go 1.21 release, we had to fork crypto/tls to add the required APIs ourselves: [qtls for Go 1.20](https://github.com/quic-go/qtls-go1-20).
This had led to a lot of pain in the Go ecosystem, and we're happy that we can rely on Go 1.21 going forward.

## Contributing

We are always happy to welcome new contributors! We have a number of self-contained issues that are suitable for first-time contributors, they are tagged with [help wanted](https://github.com/quic-go/quic-go/issues?q=is%3Aissue+is%3Aopen+label%3A%22help+wanted%22). If you have any questions, please feel free to reach out by opening an issue or leaving a comment.
