package js

import (
	"github.com/loadimpact/speedboat/loadtest"
	"gopkg.in/olebedev/go-duktape.v2"
	"testing"
)

func mustMakeJSContext() *duktape.Context {
	r := New()
	return r.newJSContext(t, id, ch)
}
