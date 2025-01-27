package common

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/chromedp/cdproto/runtime"
	"github.com/mailru/easyjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
)

func TestValueFromRemoteObject(t *testing.T) {
	t.Parallel()

	t.Run("unserializable value error", func(t *testing.T) {
		t.Parallel()

		vu := k6test.NewVU(t)
		unserializableValue := runtime.UnserializableValue("a string instead")
		remoteObject := &runtime.RemoteObject{
			Type:                "number",
			UnserializableValue: unserializableValue,
		}

		arg, err := valueFromRemoteObject(vu.Context(), remoteObject)
		require.Nil(t, arg)
		require.ErrorIs(t, UnserializableValueError{unserializableValue}, err)
	})

	t.Run("bigint parsing error", func(t *testing.T) {
		t.Parallel()

		vu := k6test.NewVU(t)
		unserializableValue := runtime.UnserializableValue("a string instead")
		remoteObject := &runtime.RemoteObject{
			Type:                "bigint",
			UnserializableValue: unserializableValue,
		}

		arg, err := valueFromRemoteObject(vu.Context(), remoteObject)

		require.Nil(t, arg)
		assert.ErrorIs(t, UnserializableValueError{unserializableValue}, err)
	})

	t.Run("float64 unserializable values", func(t *testing.T) {
		t.Parallel()

		vu := k6test.NewVU(t)
		unserializableValues := []struct {
			value    string
			expected float64
		}{
			{
				value:    "-0",
				expected: math.Float64frombits(0 | (1 << 63)),
			},
			{
				value:    "Infinity",
				expected: math.Inf(0),
			},
			{
				value:    "-Infinity",
				expected: math.Inf(-1),
			},
			{
				value:    "NaN",
				expected: math.NaN(),
			},
		}

		for _, v := range unserializableValues {
			remoteObject := &runtime.RemoteObject{
				Type:                "number",
				UnserializableValue: runtime.UnserializableValue(v.value),
			}
			arg, err := valueFromRemoteObject(vu.Context(), remoteObject)
			require.NoError(t, err)
			require.NotNil(t, arg)
			require.IsType(t, float64(0), arg)
			if v.value == "NaN" {
				require.True(t, math.IsNaN(arg.(float64)))
			} else {
				require.Equal(t, v.expected, arg.(float64))
			}
		}
	})

	t.Run("undefined", func(t *testing.T) {
		t.Parallel()

		vu := k6test.NewVU(t)
		remoteObject := &runtime.RemoteObject{
			Type: runtime.TypeUndefined,
		}

		arg, err := valueFromRemoteObject(vu.Context(), remoteObject)
		require.NoError(t, err)
		require.Nil(t, arg)
	})

	t.Run("null", func(t *testing.T) {
		t.Parallel()

		vu := k6test.NewVU(t)
		remoteObject := &runtime.RemoteObject{
			Type:    runtime.TypeObject,
			Subtype: runtime.SubtypeNull,
		}

		arg, err := valueFromRemoteObject(vu.Context(), remoteObject)
		require.NoError(t, err)
		require.Nil(t, arg)
	})

	t.Run("primitive types", func(t *testing.T) {
		t.Parallel()

		primitiveTypes := []struct {
			typ   runtime.Type
			value any
		}{
			{
				typ:   "number",
				value: float64(777), // js numbers are float64
			},
			{
				typ:   "number",
				value: float64(777.0),
			},
			{
				typ:   "string",
				value: "hello world",
			},
			{
				typ:   "boolean",
				value: true,
			},
		}

		vu := k6test.NewVU(t)
		for _, p := range primitiveTypes {
			marshalled, _ := json.Marshal(p.value)
			remoteObject := &runtime.RemoteObject{
				Type:  p.typ,
				Value: marshalled,
			}

			arg, err := valueFromRemoteObject(vu.Context(), remoteObject)

			require.Nil(t, err)
			require.IsType(t, p.value, arg)
		}
	})

	t.Run("remote object with ID", func(t *testing.T) {
		t.Parallel()

		remoteObjectID := runtime.RemoteObjectID("object_id_0123456789")
		remoteObject := &runtime.RemoteObject{
			Type:     "object",
			Subtype:  "node",
			ObjectID: remoteObjectID,
			Preview: &runtime.ObjectPreview{
				Properties: []*runtime.PropertyPreview{
					{Name: "num", Type: runtime.TypeNumber, Value: "1"},
				},
			},
		}

		vu := k6test.NewVU(t)
		val, err := valueFromRemoteObject(vu.Context(), remoteObject)
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"num": float64(1)}, val)
	})
}

func TestParseRemoteObject(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		subtype  string
		preview  *runtime.ObjectPreview
		value    easyjson.RawMessage
		expected any
		expErr   string
	}{
		{
			name: "most_types",
			preview: &runtime.ObjectPreview{
				Properties: []*runtime.PropertyPreview{
					{Name: "accessor", Type: runtime.TypeAccessor, Value: ""},
					{Name: "bigint", Type: runtime.TypeBigint, Value: "100n"},
					{Name: "bool", Type: runtime.TypeBoolean, Value: "true"},
					{Name: "fn", Type: runtime.TypeFunction, Value: ""},
					{Name: "num", Type: runtime.TypeNumber, Value: "1"},
					{Name: "str", Type: runtime.TypeString, Value: "string"},
					{Name: "strquot", Type: runtime.TypeString, Value: `"quoted string"`},
					{Name: "sym", Type: runtime.TypeSymbol, Value: "Symbol()"},
				},
			},
			expected: map[string]any{
				"accessor": "accessor",
				"bigint":   int64(100),
				"bool":     true,
				"fn":       "function()",
				"num":      float64(1),
				"str":      "string",
				"strquot":  "quoted string",
				"sym":      "Symbol()",
			},
		},
		{
			name: "nested",
			preview: &runtime.ObjectPreview{
				Properties: []*runtime.PropertyPreview{
					// We don't actually get nested ObjectPreviews from CDP.
					// I.e. the object `{nested: {one: 1}}` returns value "Object"
					// for the "nested" property, with a nil *ValuePreview. :-/
					{Name: "nested", Type: runtime.TypeObject, Value: "Object"},
				},
			},
			expected: map[string]any{
				"nested": "Object",
			},
		},
		{
			name:     "err_overflow",
			preview:  &runtime.ObjectPreview{Overflow: true},
			expected: map[string]any{},
			expErr:   "object is too large and will be parsed partially",
		},
		{
			name: "err_parsing_property",
			preview: &runtime.ObjectPreview{
				Properties: []*runtime.PropertyPreview{
					{Name: "failprop", Type: runtime.TypeObject, Value: "some"},
				},
			},
			expected: map[string]any{},
			expErr:   "parsing object property",
		},
		{
			name:     "null",
			subtype:  "null",
			value:    nil,
			expected: nil,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			remoteObject := &runtime.RemoteObject{
				Type:     "object",
				Subtype:  runtime.Subtype(tc.subtype),
				ObjectID: runtime.RemoteObjectID("object_id_0123456789"),
				Preview:  tc.preview,
				Value:    tc.value,
			}
			val, err := parseRemoteObject(remoteObject)
			assert.Equal(t, tc.expected, val)
			if tc.expErr != "" {
				assert.Contains(t, err.Error(), tc.expErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
