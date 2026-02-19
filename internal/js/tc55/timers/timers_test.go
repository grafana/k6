package timers_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/js/modulestest"
)

func newRuntime(t testing.TB) *modulestest.Runtime {
	t.Helper()
	runtime := modulestest.NewRuntime(t)

	return runtime
}

func TestSetTimeout(t *testing.T) {
	t.Parallel()

	runtime := newRuntime(t)
	rt := runtime.VU.Runtime()
	var log []string
	require.NoError(t, rt.Set("print", func(s string) { log = append(log, s) }))

	_, err := runtime.RunOnEventLoop(`
		setTimeout(()=> {
			print("in setTimeout")
		})
		print("outside setTimeout")
	`)
	require.NoError(t, err)
	require.Equal(t, []string{"outside setTimeout", "in setTimeout"}, log)
}

func TestSetUndefinedFunction(t *testing.T) {
	t.Parallel()

	runtime := newRuntime(t)
	_, err := runtime.RunOnEventLoop(`
		setTimeout(undefined)
	`)
	require.Error(t, err, "setTimeout's callback isn't a callable function")
}

func TestSetInterval(t *testing.T) {
	t.Parallel()
	runtime := newRuntime(t)

	rt := runtime.VU.Runtime()
	var log []string
	require.NoError(t, rt.Set("print", func(s string) { log = append(log, s) }))
	require.NoError(t, rt.Set("sleep10", func() { time.Sleep(10 * time.Millisecond) }))

	_, err := runtime.RunOnEventLoop(`
		var i = 0;
		let s = setInterval(()=> {
			sleep10();
			if (i>1) {
			  print("in setInterval");
			  clearInterval(s);
			}
			i++;
		}, 1);
		print("outside setInterval")
	`)
	require.NoError(t, err)
	require.Len(t, log, 2)
	require.Equal(t, "outside setInterval", log[0])
	for i, l := range log[1:] {
		require.Equal(t, "in setInterval", l, i)
	}
}

func TestSetTimeoutOrder(t *testing.T) {
	t.Parallel()
	runtime := newRuntime(t)

	rt := runtime.VU.Runtime()
	var log []string
	require.NoError(t, rt.Set("print", func(s string) { log = append(log, s) }))

	for i := range 100 {
		_, err := runtime.RunOnEventLoop(`
			setTimeout((_) => print("one"), 1);
			setTimeout((_) => print("two"), 1);
			setTimeout((_) => print("three"), 1);
			setTimeout((_) => print("last"), 20);
			setTimeout((_) => print("four"), 1);
			setTimeout((_) => print("five"), 1);
			setTimeout((_) => print("six"), 1);
			print("outside setTimeout");
		`)
		require.NoError(t, err)
		require.Equal(t, []string{"outside setTimeout", "one", "two", "three", "four", "five", "six", "last"}, log, i)
		log = log[:0]
	}
}

func TestSetIntervalOrder(t *testing.T) {
	t.Parallel()
	runtime := newRuntime(t)

	rt := runtime.VU.Runtime()
	var log []string
	require.NoError(t, rt.Set("print", func(s string) { log = append(log, s) }))

	for range 100 {
		_, err := runtime.RunOnEventLoop(`
			var one = setInterval((_) => print("one"), 1);
			var two = setInterval((_) => print("two"), 1);
			var last = setInterval((_) => {
				print("last")
				clearInterval(one);
				clearInterval(two);
				clearInterval(three);
				clearInterval(last);
			}, 10);
			var three = setInterval((_) => print("three"), 1);
			print("outside");
		`)
		require.NoError(t, err)
		runtime.EventLoop.WaitOnRegistered()
		require.GreaterOrEqual(t, len(log), 5)
		require.Equal(t, "outside", log[0])
		for i := 1; i < len(log)-1; i += 3 {
			switch len(log) - i {
			case 2:
				require.Equal(t, []string{"one"}, log[i:i+1])
			case 3:
				require.Equal(t, []string{"one", "two"}, log[i:i+2])
			default:
				require.Equal(t, []string{"one", "two", "three"}, log[i:i+3])
			}
		}
		require.Equal(t, "last", log[len(log)-1])
		log = log[:0]
	}
}

func TestSetTimeoutContextCancel(t *testing.T) {
	t.Parallel()
	runtime := newRuntime(t)

	rt := runtime.VU.Runtime()
	var log []string
	interruptChannel := make(chan struct{})
	require.NoError(t, rt.Set("print", func(s string) { log = append(log, s) }))
	require.NoError(t, rt.Set("interrupt", func() {
		select {
		case interruptChannel <- struct{}{}:
		default:
		}
	}))

	for range 2000 {
		ctx, cancel := context.WithCancel(context.Background())
		runtime.CancelContext = cancel
		runtime.VU.CtxField = ctx //nolint:fatcontext
		runtime.VU.RuntimeField.ClearInterrupt()
		const interruptMsg = "definitely an interrupt"
		sync := make(chan struct{})
		defer func() {
			cancel()
			<-sync
		}()
		go func() {
			defer close(sync)
			select {
			case <-interruptChannel:
			case <-ctx.Done():
				return
			}

			time.Sleep(time.Millisecond)
			runtime.CancelContext()
			runtime.VU.RuntimeField.Interrupt(interruptMsg)
		}()
		_, err := runtime.RunOnEventLoop(`
			(async () => {
				let poll = async (resolve, reject) => {
					await (async () => 5);
					setTimeout(poll, 1, resolve, reject);
					interrupt();
				}
				setTimeout(async () => {
					await new Promise(poll)
				}, 0)
			})()
		`)
		if err != nil {
			require.ErrorContains(t, err, interruptMsg)
		}
		require.Empty(t, log)
	}
}

func TestClearFirstTimeoutWhenMultiple(t *testing.T) {
	t.Parallel()

	runtime := newRuntime(t)
	rt := runtime.VU.Runtime()
	var log []time.Time

	start := time.Now()
	require.NoError(t, rt.Set("time", func() { log = append(log, time.Now()) }))
	_, err := runtime.RunOnEventLoop(`
		setTimeout(() => {
		   time();
		}, 1000);
		const cancelTimeout = setTimeout(() => {}, 200);
		clearTimeout(cancelTimeout);
	`)
	require.NoError(t, err)
	require.Len(t, log, 1)
	require.Greater(t, log[0].Sub(start), time.Second)
}
