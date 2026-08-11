package k6test

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// recordingTB is a minimal testing.TB that records failures without
// failing the surrounding real test.
type recordingTB struct {
	testing.TB
	failed bool
}

func (r *recordingTB) Helper() {}

func (r *recordingTB) Errorf(string, ...any) { r.failed = true }

func (r *recordingTB) FailNow() {
	r.failed = true
	runtime.Goexit()
}

// TestToPromiseNilValue guards against the nil-pointer panic from #5124:
// when RunAsync is interrupted (e.g. Abortf on IterStart failure), the
// returned sobek.Value can be nil. ToPromise must fail the test cleanly
// instead of panicking on gv.Export().
func TestToPromiseNilValue(t *testing.T) {
	t.Parallel()

	tb := &recordingTB{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = recover() }()
		ToPromise(tb, nil)
	}()
	<-done

	require.True(t, tb.failed, "ToPromise(nil) should fail the test without panicking")
}
