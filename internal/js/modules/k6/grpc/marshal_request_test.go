package grpc

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "NoSpecialFloats",
			input:    `({ a: 1, b: "x", c: [1.5, { d: true }], e: null })`,
			expected: `{"a":1,"b":"x","c":[1.5,{"d":true}],"e":null}`,
		},
		{
			name:     "TopLevel",
			input:    `NaN`,
			expected: `"NaN"`,
		},
		{
			name:     "NestedInObject",
			input:    `({ a: 1, b: { c: NaN, d: "x" }, e: [2] })`,
			expected: `{"a":1,"b":{"c":"NaN","d":"x"},"e":[2]}`,
		},
		{
			name:     "NestedInArray",
			input:    `({ a: [1, Infinity, -Infinity, 4] })`,
			expected: `{"a":[1,"Infinity","-Infinity",4]}`,
		},
		{
			name:     "NullPrototype",
			input:    `const o = Object.create(null); o.a = NaN; o.b = 1; ({ o })`,
			expected: `{"o":{"a":"NaN","b":1}}`,
		},
		{
			name: "PrototypeToJSONWithPrivateField",
			input: `class P { #v = 1; constructor() { this.f = NaN } toJSON() { return { v: this.#v, f: this.f } } };
				({ p: new P() })`,
			expected: `{"p":{"v":1,"f":"NaN"}}`,
		},
		{
			name:     "ToJSONReturnsSpecialFloat",
			input:    `({ p: { toJSON() { return 0 / 0 } } })`,
			expected: `{"p":"NaN"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rt := sobek.New()
			input, err := rt.RunString(tt.input)
			require.NoError(t, err)
			before, err := input.ToObject(rt).MarshalJSON()
			require.NoError(t, err)

			got, err := marshalRequest(rt, input.ToObject(rt))
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(got))

			// The caller's object must never be modified.
			after, err := input.ToObject(rt).MarshalJSON()
			require.NoError(t, err)
			assert.Equal(t, string(before), string(after))
		})
	}
}

func TestMarshalRequestGetterCalledOnce(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	input, err := rt.RunString(`var calls = 0; ({ get a() { calls++; return 1 }, b: NaN })`)
	require.NoError(t, err)

	got, err := marshalRequest(rt, input.ToObject(rt))
	require.NoError(t, err)
	assert.JSONEq(t, `{"a":1,"b":"NaN"}`, string(got))
	assert.Equal(t, int64(1), rt.Get("calls").ToInteger())
}

func TestMarshalRequestCyclic(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	input, err := rt.RunString(`let a = {}; a.b = { c: a }; a`)
	require.NoError(t, err)

	_, err = marshalRequest(rt, input.ToObject(rt))
	require.ErrorContains(t, err, "Converting circular structure to JSON")
}
