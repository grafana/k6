// +build !race
// Heavily influenced by the fantastic work by @dop251 for https://github.com/dop251/goja

package tc39

import "testing"

func (ctx *tc39TestCtx) runTest(name string, f func(t *testing.T)) {
	ctx.t.Run(name, func(t *testing.T) {
		t.Parallel()
		f(t)
	})
}

func (ctx *tc39TestCtx) flush() {
}
