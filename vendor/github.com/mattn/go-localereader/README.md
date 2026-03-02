# go-localereader

CodePage decoder for Windows

## Usage

```
io.Copy(os.Stdout, localereader.NewAcpReader(bytes.Reader(bytesSjis)))
```

## Installation

```
$ go get github.com/mattn/go-localereader
```

## License

MIT

## Author

Yasuhiro Matsumoto (a.k.a. mattn)
