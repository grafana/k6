package grpc

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeNumberStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
		// same reports whether the input object must be returned as is, without a copy.
		same bool
	}{
		{
			name:     "NoSpecialFloats",
			input:    `({ a: 1, b: "x", c: [1.5, { d: true }], e: null })`,
			expected: `{"a":1,"b":"x","c":[1.5,{"d":true}],"e":null}`,
			same:     true,
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
			name:     "PrototypeToJSON",
			input:    `class P { constructor() { this.x = 1 } toJSON() { return { y: 2 } } }; ({ p: new P() })`,
			expected: `{"p":{"y":2}}`,
			same:     true,
		},
		{
			name: "PrototypeToJSONWithSpecialFloat",
			input: `class P { constructor() { this.x = NaN } toJSON() { return { y: this.x } } };
				({ p: new P() })`,
			expected: `{"p":{"y":"NaN"}}`,
		},
		{
			name:     "NullPrototype",
			input:    `const o = Object.create(null); o.a = NaN; o.b = 1; ({ o })`,
			expected: `{"o":{"a":"NaN","b":1}}`,
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

			normalized, err := normalizeNumberStrings(input, rt, nil)
			require.NoError(t, err)

			got, err := normalized.ToObject(rt).MarshalJSON()
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(got))
			assert.Equal(t, tt.same, normalized.ToObject(rt).SameAs(input.ToObject(rt)))

			// The caller's object must never be modified.
			after, err := input.ToObject(rt).MarshalJSON()
			require.NoError(t, err)
			assert.Equal(t, string(before), string(after))
		})
	}
}

func TestNormalizeNumberStringsCyclic(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	input, err := rt.RunString(`let a = {}; a.b = { c: a }; a`)
	require.NoError(t, err)

	_, err = normalizeNumberStrings(input, rt, nil)
	require.ErrorContains(t, err, "cyclic reference to an object found")
}
