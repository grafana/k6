# snaker

[![Build Status](https://travis-ci.org/serenize/snaker.svg?branch=master)](https://travis-ci.org/serenize/snaker)
[![GoDoc](https://godoc.org/github.com/serenize/snaker?status.svg)](https://godoc.org/github.com/serenize/snaker)

This is a small utility to convert camel cased strings to snake case and back, except some defined words.

## QBS Usage

To replace the original toSnake and back algorithms for [https://github.com/coocood/qbs](https://github.com/coocood/qbs)
you can easily use snaker:

Import snaker
```go
import (
  github.com/coocood/qbs
  github.com/serenize/snaker
)
```

Register the snaker methods to qbs
```go
qbs.ColumnNameToFieldName = snaker.SnakeToCamel
qbs.FieldNameToColumnName = snaker.CamelToSnake
```
