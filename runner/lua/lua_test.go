package lua

import (
	"golang.org/x/net/context"
	"testing"
)

func BenchmarkRunEmpty(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := New("script.lua", "")
	for i := 0; i < b.N; i++ {
		r.Run(ctx, int64(i))
	}
}
