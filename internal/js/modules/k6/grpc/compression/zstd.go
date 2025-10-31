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

type plugin struct {
	once sync.Once
	opts atomic.Pointer[[]kz.EOption]
}

func (p *plugin) Name() string { return name }

func (p *plugin) EnsureRegistered() error {
	p.once.Do(func() {
		p.opts.Store(&[]kz.EOption{
			kz.WithEncoderLevel(kz.EncoderLevelFromZstd(1)),
			kz.WithEncoderConcurrency(1),
		})
		encoding.RegisterCompressor(zstdCompressor{p: p})
	})
	return nil
}

func (p *plugin) Configure(m map[string]any) error {
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
	p.opts.Store(&opts)
	return nil
}

func (p *plugin) CallOption() grpc.CallOption { return grpc.UseCompressor(name) }

type zstdCompressor struct{ p *plugin }

func (z zstdCompressor) Name() string { return name }
func (z zstdCompressor) Compress(w io.Writer) (io.WriteCloser, error) {
	opts := z.p.opts.Load()
	if opts == nil {
		z.p.opts.Store(&[]kz.EOption{
			kz.WithEncoderLevel(kz.EncoderLevelFromZstd(1)),
			kz.WithEncoderConcurrency(1),
		})
		opts = z.p.opts.Load()
	}
	return kz.NewWriter(w, *opts...)
}

func (z zstdCompressor) Decompress(r io.Reader) (io.Reader, error) {
	dec, err := kz.NewReader(r, kz.WithDecoderConcurrency(1))
	if err != nil {
		return nil, err
	}
	return struct{ io.ReadCloser }{dec.IOReadCloser()}, nil // EOF 시 Close()
}

func init() {
	_ = NewRegistry().Register(&plugin{})
}
