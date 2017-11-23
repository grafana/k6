# bpool [![GoDoc](https://godoc.org/github.com/oxtoacart/bpool?status.png)](https://godoc.org/github.com/oxtoacart/bpool)

Package bpool implements leaky pools of byte arrays and Buffers as bounded channels. 
It is based on the leaky buffer example from the Effective Go documentation: http://golang.org/doc/effective_go.html#leaky_buffer

bpool provides the following pool types:

* [bpool.BufferPool](https://godoc.org/github.com/oxtoacart/bpool#BufferPool)
  which provides a fixed-size pool of
  [bytes.Buffers](http://golang.org/pkg/bytes/#Buffer).
* [bpool.BytePool](https://godoc.org/github.com/oxtoacart/bpool#BytePool) which
  provides a fixed-size pool of `[]byte` slices with a pre-set width (length).
* [bpool.SizedBufferPool](https://godoc.org/github.com/oxtoacart/bpool#SizedBufferPool), 
  which is an alternative to `bpool.BufferPool` that pre-sizes the capacity of
  buffers issued from the pool and discards buffers that have grown too large
  upon return.

A common use case for this package is to use buffers to execute HTML templates
against (via ExecuteTemplate) or encode JSON into (via json.NewEncoder). This
allows you to catch any rendering or marshalling errors prior to writing to a
`http.ResponseWriter`, which helps to avoid writing incomplete or malformed data
to the response.

## Install

`go get github.com/oxtoacart/bpool`

## Documentation

See [godoc.org](http://godoc.org/github.com/oxtoacart/bpool) or use `godoc github.com/oxtoacart/bpool`

## Example

Here's a quick example for using `bpool.BufferPool`. We create a pool of the
desired size, call the `Get()` method to obtain a buffer for use, and call
`Put(buf)` to return the buffer to the pool.

```go

var bufpool *bpool.BufferPool

func main() {

    bufpool = bpool.NewBufferPool(48)

}

func someFunction() error {

     // Get a buffer from the pool
     buf := bufpool.Get()
     ...
     ...
     ...
     // Return the buffer to the pool
     bufpool.Put(buf)

     return nil
}
```

## License

Apache 2.0 Licensed. See the LICENSE file for details.

