package timers_test

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/js/modulestest"
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
	synctest.Test(t, func(t *testing.T) {
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
	})
}

func TestSetTimeoutOrder(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
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
	})
}

func TestSetIntervalOrder(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
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
	})
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
	synctest.Test(t, func(t *testing.T) {
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
		require.GreaterOrEqual(t, log[0].Sub(start), time.Second)
	})
}

func TestClearExpiredFirstTimerBeforeItsTaskRuns(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		script string
	}{
		{
			name: "clear timeout with clearTimeout",
			script: `
				var canceledTimer = setTimeout(() => {}, 10);
				sleep(20_000_000);
				clearTimeout(canceledTimer);
			`,
		},
		{
			name: "clear timeout with clearInterval",
			script: `
				var canceledTimer = setTimeout(() => {}, 10);
				sleep(20_000_000);
				clearInterval(canceledTimer);
			`,
		},
		{
			name: "clear interval with clearTimeout",
			script: `
				var canceledTimer = setInterval(() => {}, 10);
				sleep(20_000_000);
				clearTimeout(canceledTimer);
			`,
		},
		{
			name: "clear interval with clearInterval",
			script: `
				var canceledTimer = setInterval(() => {}, 10);
				sleep(20_000_000);
				clearInterval(canceledTimer);
			`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				runtime := newRuntime(t)
				rt := runtime.VU.Runtime()
				var firedAt time.Time

				require.NoError(t, rt.Set("sleep", time.Sleep))
				require.NoError(t, rt.Set("record", func() { firedAt = time.Now() }))

				for i := range 1000 {
					firedAt = time.Time{}
					start := time.Now()
					_, err := runtime.RunOnEventLoop("setTimeout(record, 1000);" + test.script)
					require.NoErrorf(t, err, "iteration %d", i)
					require.Falsef(t, firedAt.IsZero(), "iteration %d", i)
					require.GreaterOrEqualf(t, firedAt.Sub(start), time.Second, "iteration %d", i)
				}
			})
		})
	}
}

func TestClearExpiredOnlyTimerBeforeItsTaskRuns(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		runtime := newRuntime(t)
		rt := runtime.VU.Runtime()

		require.NoError(t, rt.Set("sleep", time.Sleep))

		for i := range 1000 {
			_, err := runtime.RunOnEventLoop(`
				var canceledTimer = setTimeout(() => {
					throw new Error("canceled timer ran");
				}, 10);
				sleep(20_000_000);
				clearTimeout(canceledTimer);
			`)
			require.NoErrorf(t, err, "iteration %d", i)
		}
	})
}

func TestClearedExpiredTimerDoesNotRunNewTimersEarly(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		runtime := newRuntime(t)
		rt := runtime.VU.Runtime()
		type call struct {
			name    string
			firedAt time.Time
		}
		var calls []call

		require.NoError(t, rt.Set("sleep", time.Sleep))
		require.NoError(t, rt.Set("record", func(name string) {
			calls = append(calls, call{name: name, firedAt: time.Now()})
		}))

		for i := range 1000 {
			calls = calls[:0]
			start := time.Now()
			_, err := runtime.RunOnEventLoop(`
				var canceledTimer = setTimeout(() => record("canceled"), 10);
				sleep(20_000_000);
				clearTimeout(canceledTimer);
				setTimeout(record, 100, "first");
				setTimeout(record, 200, "second");
			`)
			require.NoErrorf(t, err, "iteration %d", i)
			require.Lenf(t, calls, 2, "iteration %d", i)
			require.Equalf(t, "first", calls[0].name, "iteration %d", i)
			require.Equalf(t, "second", calls[1].name, "iteration %d", i)
			require.GreaterOrEqualf(t, calls[0].firedAt.Sub(start), 100*time.Millisecond, "iteration %d", i)
			require.GreaterOrEqualf(t, calls[1].firedAt.Sub(start), 200*time.Millisecond, "iteration %d", i)
		}
	})
}
