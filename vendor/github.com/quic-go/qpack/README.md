# QPACK

[![PkgGoDev](https://pkg.go.dev/badge/github.com/quic-go/qpack)](https://pkg.go.dev/github.com/quic-go/qpack)
[![Code Coverage](https://img.shields.io/codecov/c/github/quic-go/qpack/master.svg?style=flat-square)](https://codecov.io/gh/quic-go/qpack)
[![Fuzzing Status](https://oss-fuzz-build-logs.storage.googleapis.com/badges/quic-go.svg)](https://bugs.chromium.org/p/oss-fuzz/issues/list?sort=-opened&can=1&q=proj:quic-go)

This is a minimal QPACK ([RFC 9204](https://datatracker.ietf.org/doc/html/rfc9204)) implementation in Go. It reuses the Huffman encoder / decoder code from the [HPACK implementation in the Go standard library](https://github.com/golang/net/tree/master/http2/hpack).

It is fully interoperable with other QPACK implementations (both encoders and decoders). However, it does not support the dynamic table and relies solely on the static table and string literals (including Huffman encoding), which limits compression efficiency. If you're interested in dynamic table support, please comment on [issue #33](https://github.com/quic-go/qpack/issues/33).

## Running the Interop Tests

Install the [QPACK interop files](https://github.com/qpackers/qifs/) by running
```bash
git submodule update --init --recursive
```

Then run the tests:
```bash
go test -v ./interop
```
