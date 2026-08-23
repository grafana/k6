package k6ext

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	k6common "go.k6.io/k6/v2/js/common"
	k6modulestest "go.k6.io/k6/v2/js/modulestest"
)

func TestPanicfInterruptsRuntimeFromAnotherGoroutine(t *testing.T) {
	testRT := k6modulestest.NewRuntime(t)
	ctx := WithVU(context.Background(), testRT.VU)

	started := make(chan struct{})
	release := make(chan struct{})
	require.NoError(t, testRT.VU.Runtime().GlobalObject().Set("waitForPanic", func() {
		close(started)
		<-release
	}))

	runErr := make(chan error, 1)
	go func() {
		runErr <- testRT.EventLoop.Start(func() error {
			_, err := testRT.VU.Runtime().RunString("waitForPanic();")
			return err
		})
	}()

	<-started
	panicReturned := make(chan struct{})
	go func() {
		defer func() {
			_ = recover()
			close(panicReturned)
		}()
		Panicf(ctx, "browser event failed")
	}()
	<-panicReturned
	close(release)

	err := <-runErr
	require.EqualError(t, k6common.UnwrapSobekInterruptedError(err), "browser event failed")
}
