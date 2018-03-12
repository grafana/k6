# Source maps consumer for Golang

[![Build Status](https://travis-ci.org/go-sourcemap/sourcemap.svg)](https://travis-ci.org/go-sourcemap/sourcemap)

API docs: https://godoc.org/github.com/go-sourcemap/sourcemap.
Examples: https://godoc.org/github.com/go-sourcemap/sourcemap#pkg-examples.
Spec: https://docs.google.com/document/d/1U1RGAehQwRypUTovF1KRlpiOFze0b-_2gc6fAH0KY0k/edit.

## Installation

Install:

```shell
go get -u github.com/go-sourcemap/sourcemap
```

## Quickstart

```go
func ExampleParse() {
	mapURL := "http://code.jquery.com/jquery-2.0.3.min.map"
	resp, err := http.Get(mapURL)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	smap, err := sourcemap.Parse(mapURL, b)
	if err != nil {
		panic(err)
	}

	line, column := 5, 6789
	file, fn, line, col, ok := smap.Source(line, column)
	fmt.Println(file, fn, line, col, ok)
	// Output: http://code.jquery.com/jquery-2.0.3.js apply 4360 27 true
}
```
