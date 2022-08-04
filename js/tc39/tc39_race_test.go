//go:build race
// +build race

// Heavily influenced by the fantastic work by @dop251 for https://github.com/dop251/goja

package tc39

import (
	"testing"
)

const (
	tc39MaxTestGroupSize = 1000 // to prevent race detector complaining about too many goroutines
)

func (ctx *tc39TestCtx) runTest(name string, f func(t *testing.T)) {
	ctx.testQueue = append(ctx.testQueue, tc39Test{name: name, f: f})
	if len(ctx.testQueue) >= tc39MaxTestGroupSize {
		ctx.flush()
	}
}

func (ctx *tc39TestCtx) flush() {
	ctx.t.Run("tc39", func(t *testing.T) {
		for _, tc := range ctx.testQueue {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				tc.f(t)
			})
		}
	})
	ctx.testQueue = ctx.testQueue[:0]
}
