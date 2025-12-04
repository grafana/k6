package common

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
)

func TestEnsureErrorConstructor(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	ctor, err := EnsureErrorConstructor(rt, "FooError")
	require.NoError(t, err)
	require.NotNil(t, ctor)

	constructor, ok := sobek.AssertConstructor(ctor)
	require.True(t, ok, "FooError constructor not callable")

	instance, err := constructor(nil, rt.ToValue("boom"))
	require.NoError(t, err)

	require.Equal(t, "FooError", instance.Get("name").Export())
	require.Equal(t, "boom", instance.Get("message").Export())

	glob := rt.GlobalObject().Get("FooError")
	require.Same(t, ctor, glob)
}

func TestJSError(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	jsErr := NewJSError(JSErrorConfig{
		Constructor: "FooError",
		Name:        "Foo",
		Message:     "boom",
		Properties: map[string]interface{}{
			"code": "E_FAIL",
		},
	})

	require.Equal(t, "Foo: boom", jsErr.Error())

	value := jsErr.JSValue(rt).ToObject(rt)
	require.Equal(t, "Foo", value.Get("name").String())
	require.Equal(t, "boom", value.Get("message").String())
	require.Equal(t, "E_FAIL", value.Get("code").String())

	_, err := rt.RunString(`
		const err = new FooError("message");
		if (!(err instanceof FooError)) {
			throw new Error("instanceof failed");
		}
	`)
	require.NoError(t, err)
}

func TestExportErrorConstructor(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	exported := ExportErrorConstructor(rt, "FooError")
	require.NotNil(t, exported)

	require.NoError(t, rt.Set("FooErrorProxy", exported))

	_, err := rt.RunString(`
		const err = new FooErrorProxy("boom");
		if (!(err instanceof FooErrorProxy)) {
			throw new Error("proxy constructor mismatch");
		}
		if (!(err instanceof FooError)) {
			throw new Error("instanceof FooError failed");
		}
	`)
	require.NoError(t, err)
}

func TestExportNamedError(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	exportFn := ExportNamedError("FooError")
	proxy := exportFn(rt)
	require.NotNil(t, proxy)

	require.NoError(t, rt.Set("FooNamedError", proxy))

	_, err := rt.RunString(`
		const err = new FooNamedError("boom");
		if (!(err instanceof FooNamedError)) {
			throw new Error("proxy constructor mismatch");
		}
		if (!(err instanceof FooError)) {
			throw new Error("instanceof FooError failed");
		}
	`)
	require.NoError(t, err)
}

func TestJSErrorNilHandling(t *testing.T) {
	t.Parallel()

	rt := sobek.New()

	var nilErr *JSError
	require.Equal(t, "", nilErr.Error())
	require.True(t, sobek.IsNull(nilErr.JSValue(rt)))
}

func TestJSErrorInstanceofBaseError(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	jsErr := NewJSError(JSErrorConfig{
		Constructor: "CustomError",
		Message:     "test message",
	})

	_ = jsErr.JSValue(rt)

	_, err := rt.RunString(`
		const e = new CustomError("msg");
		if (!(e instanceof Error)) {
			throw new Error("custom error should be instanceof Error");
		}
		if (!(e instanceof CustomError)) {
			throw new Error("should be instanceof CustomError");
		}
	`)
	require.NoError(t, err)
}

func TestJSErrorMessageWithSpecialCharacters(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		message string
	}{
		{"newlines", "line1\nline2\nline3"},
		{"quotes", `message with "double" and 'single' quotes`},
		{"unicode", "unicode: 你好世界 🚀"},
		{"backslashes", `path\to\file`},
		{"tabs", "col1\tcol2\tcol3"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rt := sobek.New()
			jsErr := NewJSError(JSErrorConfig{
				Constructor: "TestError",
				Message:     tc.message,
			})

			obj := jsErr.JSValue(rt).ToObject(rt)
			require.Equal(t, tc.message, obj.Get("message").String())
		})
	}
}

func TestJSErrorStringFormats(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		constructor string
		errName     string
		message     string
		expected    string
	}{
		{
			name:        "all fields",
			constructor: "FooError",
			errName:     "Foo",
			message:     "boom",
			expected:    "Foo: boom",
		},
		{
			name:        "no name",
			constructor: "FooError",
			errName:     "",
			message:     "boom",
			expected:    "boom",
		},
		{
			name:        "no message",
			constructor: "FooError",
			errName:     "Foo",
			message:     "",
			expected:    "Foo",
		},
		{
			name:        "only constructor",
			constructor: "FooError",
			errName:     "",
			message:     "",
			expected:    "FooError",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			jsErr := NewJSError(JSErrorConfig{
				Constructor: tc.constructor,
				Name:        tc.errName,
				Message:     tc.message,
			})

			require.Equal(t, tc.expected, jsErr.Error())
		})
	}
}

func TestEnsureErrorConstructorIdempotent(t *testing.T) {
	t.Parallel()

	rt := sobek.New()

	// Call multiple times - should return the same constructor
	ctor1, err := EnsureErrorConstructor(rt, "IdempotentError")
	require.NoError(t, err)

	ctor2, err := EnsureErrorConstructor(rt, "IdempotentError")
	require.NoError(t, err)

	require.Same(t, ctor1, ctor2, "EnsureErrorConstructor should return the same constructor on repeated calls")
}

func TestEnsureErrorConstructorEmptyName(t *testing.T) {
	t.Parallel()

	rt := sobek.New()

	// Empty name should default to "Error"
	ctor, err := EnsureErrorConstructor(rt, "")
	require.NoError(t, err)

	// Should return the built-in Error constructor
	builtinError := rt.Get("Error")
	require.Equal(t, builtinError, ctor)
}
