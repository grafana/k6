package timers_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/modules/k6/timers"
	"go.k6.io/k6/js/modulestest"
)

func newRuntime(t testing.TB) *modulestest.Runtime {
	t.Helper()
	runtime := modulestest.NewRuntime(t)
	err := runtime.SetupModuleSystem(map[string]any{"k6/x/timers": timers.New()}, nil, nil)
	require.NoError(t, err)
	return runtime
}

func TestSetTimeout(t *testing.T) {
	t.Parallel()

	runtime := newRuntime(t)
	rt := runtime.VU.Runtime()
	var log []string
	require.NoError(t, rt.Set("print", func(s string) { log = append(log, s) }))

	_, err := runtime.RunOnEventLoop(`
		let timers = require("k6/x/timers");
		timers.setTimeout(()=> {
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
		let timers = require("k6/x/timers");
		timers.setTimeout(undefined)
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
		let timers = require("k6/x/timers");
		var i = 0;
		let s = timers.setInterval(()=> {
			sleep10();
			if (i>1) {
			  print("in setInterval");
			  timers.clearInterval(s);
			}
			i++;
		}, 1);
		print("outside setInterval")
	`)
	require.NoError(t, err)
	require.Equal(t, len(log), 2)
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

	_, err := rt.RunString(`globalThis.setTimeout = require("k6/x/timers").setTimeout;`)
	require.NoError(t, err)

	for i := 0; i < 100; i++ {
		_, err = runtime.RunOnEventLoop(`
			setTimeout((_) => print("one"), 1);
			setTimeout((_) => print("two"), 1);
			setTimeout((_) => print("three"), 1);
			setTimeout((_) => print("last"), 10);
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

	_, err := rt.RunString(`globalThis.setInterval = require("k6/x/timers").setInterval;`)
	require.NoError(t, err)

	_, err = rt.RunString(`globalThis.clearInterval = require("k6/x/timers").clearInterval;`)
	require.NoError(t, err)

	for i := 0; i < 100; i++ {
		_, err = runtime.RunOnEventLoop(`
			var one = setInterval((_) => print("one"), 1);
			var two = setInterval((_) => print("two"), 1);
			var last = setInterval((_) => {
				print("last")
				clearInterval(one);
				clearInterval(two);
				clearInterval(three);
				clearInterval(last);
			}, 4);
			var three = setInterval((_) => print("three"), 1);
			print("outside");
		`)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(log), 5)
		require.Equal(t, log[0], "outside")
		for i := 1; i < len(log)-1; i += 3 {
			switch len(log) - i {
			case 2:
				require.Equal(t, log[i:i+1], []string{"one"})
			case 3:
				require.Equal(t, log[i:i+2], []string{"one", "two"})
			default:
				require.Equal(t, log[i:i+3], []string{"one", "two", "three"})
			}
		}
		require.Equal(t, log[len(log)-1], "last")
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

	_, err := rt.RunString(`globalThis.setTimeout = require("k6/x/timers").setTimeout;`)
	require.NoError(t, err)

	for i := 0; i < 2000; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		runtime.CancelContext = cancel
		runtime.VU.CtxField = ctx
		runtime.VU.RuntimeField.ClearInterrupt()
		const interruptMsg = "definitely an interrupt"
		go func() {
			<-interruptChannel
			time.Sleep(time.Millisecond)
			runtime.CancelContext()
			runtime.VU.RuntimeField.Interrupt(interruptMsg)
		}()
		_, err = runtime.RunOnEventLoop(`
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
