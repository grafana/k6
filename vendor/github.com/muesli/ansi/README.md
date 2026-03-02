# ansi

[![Latest Release](https://img.shields.io/github/release/muesli/ansi.svg)](https://github.com/muesli/ansi/releases)
[![Build Status](https://github.com/muesli/ansi/workflows/build/badge.svg)](https://github.com/muesli/ansi/actions)
[![Coverage Status](https://coveralls.io/repos/github/muesli/ansi/badge.svg?branch=master)](https://coveralls.io/github/muesli/ansi?branch=master)
[![Go ReportCard](https://goreportcard.com/badge/muesli/ansi)](https://goreportcard.com/report/muesli/ansi)
[![GoDoc](https://godoc.org/github.com/golang/gddo?status.svg)](https://pkg.go.dev/github.com/muesli/ansi)

Raw ANSI sequence helpers

## ANSI Writer

```go
import "github.com/muesli/ansi"

w := ansi.Writer{Forward: os.Stdout}
w.Write([]byte("\x1b[31mHello, world!\x1b[0m"))
w.Close()
```

## Compressor

The ANSI compressor eliminates unnecessary/redundant ANSI sequences.

```go
import "github.com/muesli/ansi/compressor"

w := compressor.Writer{Forward: os.Stdout}
w.Write([]byte("\x1b[31mHello, world!\x1b[0m"))
w.Close()
```
