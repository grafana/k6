package tracing

import (
	"net/http"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/modulestest"
)

func TestClientGetOrCreateParams(t *testing.T) {
	t.Parallel()

	t.Run("a provided body and params fn(fst, body, {}) leaves params untouched", func(t *testing.T) {
		t.Parallel()

		testSetup := modulestest.NewRuntime(t)
		client := Client{vu: testSetup.VU}
		wantBody := testSetup.VU.Runtime().NewObject()
		wantParams := testSetup.VU.Runtime().NewObject()
		headers := testSetup.VU.Runtime().NewObject()
		err := headers.Set("Content-Type", "application/json")
		require.NoError(t, err)
		err = wantParams.Set("headers", headers)
		require.NoError(t, err)

		gotArgs, gotParams, gotErr := client.getOrCreateParams(http.MethodPost, wantBody, wantParams)

		assert.NoError(t, gotErr)
		assert.NotNil(t, gotParams)
		assert.NotNil(t, gotArgs)
		assert.Len(t, gotArgs, 2)
		assert.Equal(t, wantBody, gotArgs[0])
		assert.Equal(t, wantParams, gotArgs[1])
	})

	t.Run("a provided body and null params arg fn(fst, body, null) intializes params", func(t *testing.T) {
		t.Parallel()

		testSetup := modulestest.NewRuntime(t)
		client := &Client{vu: testSetup.VU}
		wantBody := testSetup.VU.Runtime().NewObject()

		gotArgs, gotParams, gotErr := client.getOrCreateParams(http.MethodPost, wantBody, goja.Null())

		assert.NoError(t, gotErr)
		assert.NotNil(t, gotParams)
		assert.NotNil(t, gotArgs)
		assert.Len(t, gotArgs, 2)
		assert.Equal(t, wantBody, gotArgs[0])
		assert.Equal(t, gotParams, gotArgs[1])
	})

	t.Run("a provided body and undefined params arg fn(fst, body, undefined) intializes params", func(t *testing.T) {
		t.Parallel()

		testSetup := modulestest.NewRuntime(t)
		client := &Client{vu: testSetup.VU}
		wantBody := testSetup.VU.Runtime().NewObject()

		gotArgs, gotParams, gotErr := client.getOrCreateParams(http.MethodPost, wantBody, goja.Undefined())

		assert.NoError(t, gotErr)
		assert.NotNil(t, gotParams)
		assert.NotNil(t, gotArgs)
		assert.Len(t, gotArgs, 2)
		assert.Equal(t, wantBody, gotArgs[0])
		assert.Equal(t, gotParams, gotArgs[1])
	})

	t.Run("a provided body and nil params arg fn(fst, body) intializes params", func(t *testing.T) {
		t.Parallel()

		testSetup := modulestest.NewRuntime(t)
		client := &Client{vu: testSetup.VU}
		wantBody := testSetup.VU.Runtime().NewObject()

		gotArgs, gotParams, gotErr := client.getOrCreateParams(http.MethodPost, wantBody, nil)

		assert.NoError(t, gotErr)
		assert.NotNil(t, gotParams)
		assert.NotNil(t, gotArgs)
		assert.Len(t, gotArgs, 2)
		assert.Equal(t, wantBody, gotArgs[0])
		assert.Equal(t, gotParams, gotArgs[1])
	})

	t.Run("a provided body and no params arg fn(fst, body) intializes params", func(t *testing.T) {
		t.Parallel()

		testSetup := modulestest.NewRuntime(t)
		client := &Client{vu: testSetup.VU}
		wantBody := testSetup.VU.Runtime().NewObject()

		gotArgs, gotParams, gotErr := client.getOrCreateParams(http.MethodPost, wantBody)

		assert.NoError(t, gotErr)
		assert.NotNil(t, gotParams)
		assert.NotNil(t, gotArgs)
		assert.Len(t, gotArgs, 2)
		assert.Equal(t, wantBody, gotArgs[0])
		assert.Equal(t, gotParams, gotArgs[1])
	})

	t.Run("a provided nil body and no params arg fn(fst, null) intializes params", func(t *testing.T) {
		t.Parallel()

		testSetup := modulestest.NewRuntime(t)
		client := &Client{vu: testSetup.VU}

		gotArgs, gotParams, gotErr := client.getOrCreateParams(http.MethodPost, nil)

		assert.NoError(t, gotErr)
		assert.NotNil(t, gotParams)
		assert.NotNil(t, gotArgs)
		assert.Len(t, gotArgs, 2)
		assert.Nil(t, gotArgs[0])
		assert.Equal(t, gotParams, gotArgs[1])
	})

	t.Run("a provided null params argument fn(fst, null) should initialize it", func(t *testing.T) {
		t.Parallel()

		testSetup := modulestest.NewRuntime(t)
		client := &Client{vu: testSetup.VU}

		gotArgs, gotParams, gotErr := client.getOrCreateParams(http.MethodGet, goja.Null())

		assert.NoError(t, gotErr)
		assert.NotNil(t, gotParams)
		assert.NotNil(t, gotArgs)
		assert.Len(t, gotArgs, 1)
		assert.Equal(t, gotParams, gotArgs[0])
	})

	t.Run("a provided undefined params argument fn(fst, undefined) should initialize it", func(t *testing.T) {
		t.Parallel()

		testSetup := modulestest.NewRuntime(t)
		client := &Client{vu: testSetup.VU}

		gotArgs, gotParams, gotErr := client.getOrCreateParams(http.MethodGet, goja.Undefined())

		assert.NoError(t, gotErr)
		assert.NotNil(t, gotParams)
		assert.NotNil(t, gotArgs)
		assert.Len(t, gotArgs, 1)
		assert.Equal(t, gotParams, gotArgs[0])
	})

	t.Run("a provided nil params argument fn(fst) should initialize it", func(t *testing.T) {
		t.Parallel()

		testSetup := modulestest.NewRuntime(t)
		client := &Client{vu: testSetup.VU}

		gotArgs, gotParams, gotErr := client.getOrCreateParams(http.MethodGet, nil)

		assert.NoError(t, gotErr)
		assert.NotNil(t, gotParams)
		assert.NotNil(t, gotArgs)
		assert.Len(t, gotArgs, 1)
		assert.Equal(t, gotParams, gotArgs[0])
	})

	t.Run("a provided params argument fn(fst, {}) should leave it untouched", func(t *testing.T) {
		t.Parallel()

		testSetup := modulestest.NewRuntime(t)
		client := &Client{vu: testSetup.VU}
		headers := testSetup.VU.Runtime().NewObject()
		err := headers.Set("test-header", "test-value")
		require.NoError(t, err)
		wantParams := testSetup.VU.Runtime().NewObject()
		err = wantParams.Set("headers", headers)
		require.NoError(t, err)

		gotArgs, gotParams, gotErr := client.getOrCreateParams(http.MethodGet, wantParams)

		assert.NoError(t, gotErr)
		assert.NotNil(t, gotParams)
		assert.NotNil(t, gotArgs)
		assert.Len(t, gotArgs, 1)
		assert.Equal(t, gotArgs[0], wantParams)
		assert.True(t, gotParams == wantParams)
	})

	t.Run("no arguments beyond first fn(fst) should initialize a params argument", func(t *testing.T) {
		t.Parallel()

		testSetup := modulestest.NewRuntime(t)
		client := &Client{vu: testSetup.VU}

		// the http.get method has a single argument, besides url,
		// which is the optional gotParams object. We use it here to
		// verify that we create a default gotParams object under the
		// hood.
		gotArgs, gotParams, gotErr := client.getOrCreateParams(http.MethodGet)

		assert.NoError(t, gotErr)
		assert.NotNil(t, gotParams)
		assert.NotNil(t, gotArgs)
		assert.Len(t, gotArgs, 2)
		assert.Equal(t, gotArgs[0], goja.Null())
		assert.Equal(t, gotArgs[1], gotParams)
	})
}

func TestClientGetOrCreateHeaders(t *testing.T) {
	t.Parallel()

	t.Run("params object with headers fn(..., { headers: {...} }) should return them", func(t *testing.T) {
		t.Parallel()

		testSetup := modulestest.NewRuntime(t)
		client := &Client{vu: testSetup.VU}
		params := testSetup.VU.Runtime().NewObject()
		headers := testSetup.VU.Runtime().NewObject()
		err := params.Set("headers", headers)
		require.NoError(t, err)

		gotHeaders, gotErr := client.getOrCreateHeaders(params)

		assert.NoError(t, gotErr)
		assert.Equal(t, gotHeaders, params.Get("headers"))
	})

	t.Run("params object without headers fn(..., { }) should initialize some", func(t *testing.T) {
		t.Parallel()

		testSetup := modulestest.NewRuntime(t)
		client := &Client{vu: testSetup.VU}
		wantParams := testSetup.VU.Runtime().NewObject()

		gotHeaders, gotErr := client.getOrCreateHeaders(wantParams)

		assert.NoError(t, gotErr)
		assert.NotNil(t, wantParams.Get("headers"))
		assert.NotNil(t, gotHeaders)
	})

	t.Run("params object with undefined headers fn(..., { headers: undefined }) should set some", func(t *testing.T) {
		t.Parallel()

		testSetup := modulestest.NewRuntime(t)
		client := &Client{vu: testSetup.VU}
		wantParams := testSetup.VU.Runtime().NewObject()
		err := wantParams.Set("headers", goja.Undefined())
		require.NoError(t, err)

		gotHeaders, gotErr := client.getOrCreateHeaders(wantParams)

		assert.NoError(t, gotErr)
		assert.NotNil(t, wantParams.Get("headers"))
		assert.NotNil(t, gotHeaders)
	})

	t.Run("params object with null headers fn(..., { headers: null }) should set some", func(t *testing.T) {
		t.Parallel()

		testSetup := modulestest.NewRuntime(t)
		client := &Client{vu: testSetup.VU}
		wantParams := testSetup.VU.Runtime().NewObject()
		err := wantParams.Set("headers", goja.Null())
		require.NoError(t, err)

		gotHeaders, gotErr := client.getOrCreateHeaders(wantParams)

		assert.NoError(t, gotErr)
		assert.NotNil(t, wantParams.Get("headers"))
		assert.NotNil(t, gotHeaders)
	})
}
