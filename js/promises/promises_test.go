package promises

import (
	"errors"
	"testing"

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

	t.Run("should reject Go errors without wrapping them", func(t *testing.T) {
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
						err => { if (String(err) !== "plain error") { throw "unexpected error: " + err } },
					)
			`)
			go func() {
				reject(errors.New("plain error"))
			}()

			return err
		})

		assert.NoError(t, err)
	})
}
