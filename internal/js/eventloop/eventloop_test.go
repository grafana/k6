package eventloop_test

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/js/eventloop"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modulestest"
)

func TestBasicEventLoop(t *testing.T) {
	t.Parallel()
	loop := eventloop.New(&modulestest.VU{RuntimeField: sobek.New()})
	var ran int
	f := func() error { //nolint:unparam
		ran++
		return nil
	}
	require.NoError(t, loop.Start(f))
	require.Equal(t, 1, ran)
	require.NoError(t, loop.Start(f))
	require.Equal(t, 2, ran)
	require.Error(t, loop.Start(func() error {
		_ = f()
		loop.RegisterCallback()(f)
		return errors.New("something")
	}))
	require.Equal(t, 3, ran)
}

func TestEventLoopRegistered(t *testing.T) {
	t.Parallel()
	loop := eventloop.New(&modulestest.VU{RuntimeField: sobek.New()})
	var ran int
	f := func() error {
		ran++
		r := loop.RegisterCallback()
		go func() {
			time.Sleep(time.Second)
			r(func() error {
				ran++
				return nil
			})
		}()
		return nil
	}
	start := time.Now()
	require.NoError(t, loop.Start(f))
	took := time.Since(start)
	require.Equal(t, 2, ran)
	require.Less(t, time.Second, took)
	require.Greater(t, time.Second+time.Millisecond*100, took)
}

func TestEventLoopWaitOnRegistered(t *testing.T) {
	t.Parallel()
	var ran int
	loop := eventloop.New(&modulestest.VU{RuntimeField: sobek.New()})
	f := func() error {
		ran++
		r := loop.RegisterCallback()
		go func() {
			time.Sleep(time.Second)
			r(func() error {
				ran++
				return nil
			})
		}()
		return fmt.Errorf("expected")
	}
	start := time.Now()
	require.Error(t, loop.Start(f))
	took := time.Since(start)
	loop.WaitOnRegistered()
	took2 := time.Since(start)
	require.Equal(t, 2, ran)
	require.Greater(t, time.Millisecond*50, took)
	require.Less(t, time.Second, took2)
	require.Greater(t, time.Second+time.Millisecond*100, took2)
}

func TestEventLoopAllCallbacksGetCalled(t *testing.T) {
	t.Parallel()
	sleepTime := time.Millisecond * 500
	loop := eventloop.New(&modulestest.VU{RuntimeField: sobek.New()})
	var called int64
	f := func() error {
		for i := 0; i < 100; i++ {
			bad := i == 99
			r := loop.RegisterCallback()

			go func() {
				if !bad {
					time.Sleep(sleepTime)
				}
				r(func() error {
					if bad {
						return errors.New("something")
					}
					atomic.AddInt64(&called, 1)
					return nil
				})
			}()
		}
		return fmt.Errorf("expected")
	}
	for i := 0; i < 3; i++ {
		called = 0
		start := time.Now()
		require.Error(t, loop.Start(f))
		took := time.Since(start)
		loop.WaitOnRegistered()
		took2 := time.Since(start)
		require.Greater(t, time.Millisecond*50, took)
		require.Less(t, sleepTime, took2)
		require.Greater(t, sleepTime+time.Millisecond*100, took2)
		require.EqualValues(t, called, 99)
	}
}

func TestEventLoopPanicOnDoubleCallback(t *testing.T) {
	t.Parallel()
	loop := eventloop.New(&modulestest.VU{RuntimeField: sobek.New()})
	var ran int
	f := func() error {
		ran++
		r := loop.RegisterCallback()
		go func() {
			time.Sleep(time.Second)
			r(func() error {
				ran++
				return nil
			})

			require.Panics(t, func() { r(func() error { return nil }) })
		}()
		return nil
	}
	start := time.Now()
	require.NoError(t, loop.Start(f))
	took := time.Since(start)
	require.Equal(t, 2, ran)
	require.Less(t, time.Second, took)
	require.Greater(t, time.Second+time.Millisecond*100, took)
}

func TestEventLoopRejectUndefined(t *testing.T) {
	t.Parallel()
	vu := &modulestest.VU{RuntimeField: sobek.New()}
	loop := eventloop.New(vu)
	err := loop.Start(func() error {
		_, err := vu.Runtime().RunString("Promise.reject()")
		return err
	})
	loop.WaitOnRegistered()
	require.EqualError(t, err, "Uncaught (in promise) undefined")
}

func TestEventLoopRejectString(t *testing.T) {
	t.Parallel()
	vu := &modulestest.VU{RuntimeField: sobek.New()}
	loop := eventloop.New(vu)
	err := loop.Start(func() error {
		_, err := vu.Runtime().RunString("Promise.reject('some string')")
		return err
	})
	loop.WaitOnRegistered()
	require.EqualError(t, err, "Uncaught (in promise) some string")
}

func TestEventLoopRejectSyntaxError(t *testing.T) {
	t.Parallel()
	vu := &modulestest.VU{RuntimeField: sobek.New()}
	loop := eventloop.New(vu)
	err := loop.Start(func() error {
		_, err := vu.Runtime().RunString("Promise.resolve().then(()=> {some.syntax.error})")
		return err
	})
	loop.WaitOnRegistered()
	require.EqualError(t, err, "Uncaught (in promise) ReferenceError: some is not defined\n\tat <eval>:1:30(1)\n")
}

func TestEventLoopRejectGoError(t *testing.T) {
	t.Parallel()
	vu := &modulestest.VU{RuntimeField: sobek.New()}
	loop := eventloop.New(vu)
	rt := vu.Runtime()
	require.NoError(t, rt.Set("f", rt.ToValue(func() error {
		return errors.New("some error")
	})))
	err := loop.Start(func() error {
		_, err := vu.Runtime().RunString("Promise.resolve().then(()=> {f()})")
		return err
	})
	loop.WaitOnRegistered()
	require.EqualError(t, err, "Uncaught (in promise) GoError: some error\n\tat go.k6.io/k6/internal/js/eventloop_test.TestEventLoopRejectGoError.func1 (native)\n\tat <eval>:1:31(2)\n")
}

func TestEventLoopRejectThrow(t *testing.T) {
	t.Parallel()
	vu := &modulestest.VU{RuntimeField: sobek.New()}
	loop := eventloop.New(vu)
	rt := vu.Runtime()
	require.NoError(t, rt.Set("f", rt.ToValue(func() error {
		common.Throw(rt, errors.New("throw error"))
		return nil
	})))
	err := loop.Start(func() error {
		_, err := vu.Runtime().RunString("Promise.resolve().then(()=> {f()})")
		return err
	})
	loop.WaitOnRegistered()
	require.EqualError(t, err, "Uncaught (in promise) GoError: throw error\n\tat go.k6.io/k6/internal/js/eventloop_test.TestEventLoopRejectThrow.func1 (native)\n\tat <eval>:1:31(2)\n")
}

func TestEventLoopAsyncAwait(t *testing.T) {
	t.Parallel()
	vu := &modulestest.VU{RuntimeField: sobek.New()}
	loop := eventloop.New(vu)
	err := loop.Start(func() error {
		_, err := vu.Runtime().RunString(`
        async function a() {
            some.error.here
        }
        Promise.resolve().then(async () => {
            await a();
        })
        `)
		return err
	})
	loop.WaitOnRegistered()
	require.EqualError(t, err, "Uncaught (in promise) ReferenceError: some is not defined\n\tat a (<eval>:3:13(1))\n\tat <eval>:6:20(2)\n")
}
