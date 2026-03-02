package localereader

import (
	"bytes"
	"io"
)

func NewReader(r io.Reader) io.Reader {
	return newReader(r)
}

func UTF8(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	n, err := io.Copy(&buf, newReader(bytes.NewReader(b)))
	if err != nil {
		return nil, err
	}
	return buf.Bytes()[:n], nil
}
