package promises

import (
	"errors"
	"fmt"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("should resolve", func(t *testing.T) {
		t.Parallel()

		runtime := modulestest.NewRuntime(t)
		err := runtime.EventLoop.Start(func() error {
			promise, resolve, _ := New(runtime.VU)
			err := runtime.VU.Runtime().Set("promise", promise)
			require.NoError(t, err)

			_, err = runtime.VU.Runtime().RunString(`
				promise
					.then(
						res => { if (res !== "resolved") { throw "unexpected promise resolution with result: " + res } },
						err => { throw "unexpected error: " + err },
					)
			`)
			go func() {
				resolve("resolved")
			}()

			return err
		})

		assert.NoError(t, err)
	})

	t.Run("should reject", func(t *testing.T) {
		t.Parallel()

		runtime := modulestest.NewRuntime(t)
		err := runtime.EventLoop.Start(func() error {
			promise, _, reject := New(runtime.VU)
			err := runtime.VU.Runtime().Set("promise", promise)
			require.NoError(t, err)

			_, err = runtime.VU.Runtime().RunString(`
				promise
					.then(
						res => { throw "unexpected promise resolution with result: " + res },
						err => { if (err !== "rejected") { throw "unexpected error: " + err } },
					)
			`)
			go func() {
				reject("rejected")
			}()

			return err
		})

		assert.NoError(t, err)
	})
}

func TestRejectGoError(t *testing.T) {
	t.Parallel()

	original := errors.New("connection refused")
	for _, reason := range []error{original, fmt.Errorf("connecting: %w", original)} {
		t.Run(reason.Error(), func(t *testing.T) {
			t.Parallel()

			runtime := modulestest.NewRuntime(t)
			require.NoError(t, runtime.VU.Runtime().Set("operation", func() *sobek.Promise {
				promise, _, reject := New(runtime.VU)
				go func() { reject(reason) }()
				return promise
			}))

			value, err := runtime.RunOnEventLoop(`operation().catch(error =>
				[error.name, error.message, error instanceof Error, error.value])`)
			require.NoError(t, err)
			promise := value.Export().(*sobek.Promise)
			require.Equal(t, sobek.PromiseStateFulfilled, promise.State())
			assert.Equal(t, []any{"GoError", reason.Error(), true, reason}, promise.Result().Export())

			_, err = runtime.RunOnEventLoop(`operation()`)
			require.ErrorContains(t, err, "Uncaught (in promise) GoError: "+reason.Error())
			require.ErrorIs(t, err, original)
		})
	}
}

func TestRejectPreservesValue(t *testing.T) {
	t.Parallel()

	for _, expression := range []string{`"rejected"`, `42`, `null`, `undefined`, `({message: "failure"})`, `new Error("failure")`} {
		t.Run(expression, func(t *testing.T) {
			t.Parallel()

			runtime := modulestest.NewRuntime(t)
			reason, err := runtime.VU.Runtime().RunString(expression)
			require.NoError(t, err)
			err = runtime.EventLoop.Start(func() error {
				promise, _, reject := New(runtime.VU)
				go func() { reject(reason) }()
				require.NoError(t, runtime.VU.Runtime().Set("promise", promise))
				require.NoError(t, runtime.VU.Runtime().Set("reason", reason))
				_, err := runtime.VU.Runtime().RunString(`promise.catch(error => {
					if (error !== reason) throw new Error("rejection value changed");
				})`)
				return err
			})
			runtime.EventLoop.WaitOnRegistered()
			require.NoError(t, err)
		})
	}
}
