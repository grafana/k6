package grpccompress

import (
	"io"
	"sync"
	"sync/atomic"

	kz "github.com/klauspost/compress/zstd"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
)

const name = "zstd"

type plugin struct{}

//nolint:gochecknoglobals
var (
	registerOnce sync.Once
	encOpts      atomic.Value
)

func (plugin) Name() string { return name }

func (plugin) EnsureRegistered() error {
	registerOnce.Do(func() {
		encOpts.Store([]kz.EOption{kz.WithEncoderLevel(kz.SpeedDefault), kz.WithEncoderConcurrency(1)})
		encoding.RegisterCompressor(zstdCompressor{})
	})
	return nil
}

func (plugin) Configure(m map[string]any) error {
	opts := []kz.EOption{}
	if v, ok := m["level"].(int); ok && v > 0 {
		opts = append(opts, kz.WithEncoderLevel(kz.EncoderLevelFromZstd(v)))
	}
	if c, ok := m["concurrency"].(int); ok && c > 0 {
		opts = append(opts, kz.WithEncoderConcurrency(c))
	} else {
		opts = append(opts, kz.WithEncoderConcurrency(1))
	}
	if ws, ok := m["windowSize"].(int); ok && ws > 0 {
		opts = append(opts, kz.WithWindowSize(ws))
	}
	if b, ok := m["noEntropy"].(bool); ok && b {
		opts = append(opts, kz.WithNoEntropyCompression(true))
	}
	if len(opts) == 0 {
		opts = append(opts, kz.WithEncoderLevel(kz.SpeedDefault), kz.WithEncoderConcurrency(1))
	}
	encOpts.Store(opts)
	return nil
}

func (plugin) CallOption() grpc.CallOption { return grpc.UseCompressor(name) }

type zstdCompressor struct{}

func (zstdCompressor) Name() string { return name }
func (zstdCompressor) Compress(w io.Writer) (io.WriteCloser, error) {
	v, ok := encOpts.Load().([]kz.EOption)
	if !ok || v == nil {
		return kz.NewWriter(w)
	}
	return kz.NewWriter(w, v...)
}

func (zstdCompressor) Decompress(r io.Reader) (io.Reader, error) {
	dec, err := kz.NewReader(r, kz.WithDecoderConcurrency(1))
	if err != nil {
		return nil, err
	}
	return struct{ io.ReadCloser }{dec.IOReadCloser()}, nil // EOF 시 Close()
}

func init() { Register(plugin{}) }
