package js

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/modulestest"
)

func TestBasicEventLoop(t *testing.T) {
	t.Parallel()
	loop := newEventLoop(&modulestest.VU{RuntimeField: goja.New()})
	var ran int
	f := func() error { //nolint:unparam
		ran++
		return nil
	}
	require.NoError(t, loop.start(f))
	require.Equal(t, 1, ran)
	require.NoError(t, loop.start(f))
	require.Equal(t, 2, ran)
	require.Error(t, loop.start(func() error {
		_ = f()
		loop.registerCallback()(f)
		return errors.New("something")
	}))
	require.Equal(t, 3, ran)
}

func TestEventLoopRegistered(t *testing.T) {
	t.Parallel()
	loop := newEventLoop(&modulestest.VU{RuntimeField: goja.New()})
	var ran int
	f := func() error {
		ran++
		r := loop.registerCallback()
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
	require.NoError(t, loop.start(f))
	took := time.Since(start)
	require.Equal(t, 2, ran)
	require.Less(t, time.Second, took)
	require.Greater(t, time.Second+time.Millisecond*100, took)
}

func TestEventLoopWaitOnRegistered(t *testing.T) {
	t.Parallel()
	var ran int
	loop := newEventLoop(&modulestest.VU{RuntimeField: goja.New()})
	f := func() error {
		ran++
		r := loop.registerCallback()
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
	require.Error(t, loop.start(f))
	took := time.Since(start)
	loop.waitOnRegistered()
	took2 := time.Since(start)
	require.Equal(t, 1, ran)
	require.Greater(t, time.Millisecond*50, took)
	require.Less(t, time.Second, took2)
	require.Greater(t, time.Second+time.Millisecond*100, took2)
}

func TestEventLoopReuse(t *testing.T) {
	t.Parallel()
	sleepTime := time.Millisecond * 500
	loop := newEventLoop(&modulestest.VU{RuntimeField: goja.New()})
	f := func() error {
		for i := 0; i < 100; i++ {
			bad := i == 17
			r := loop.registerCallback()

			go func() {
				if !bad {
					time.Sleep(sleepTime)
				}
				r(func() error {
					if bad {
						return errors.New("something")
					}
					panic("this should never execute")
				})
			}()
		}
		return fmt.Errorf("expected")
	}
	for i := 0; i < 3; i++ {
		start := time.Now()
		require.Error(t, loop.start(f))
		took := time.Since(start)
		loop.waitOnRegistered()
		took2 := time.Since(start)
		require.Greater(t, time.Millisecond*50, took)
		require.Less(t, sleepTime, took2)
		require.Greater(t, sleepTime+time.Millisecond*100, took2)
	}
}
