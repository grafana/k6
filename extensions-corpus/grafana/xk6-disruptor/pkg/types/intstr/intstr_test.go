package intstr

import (
	"testing"
)

func Test_IntStrFrom(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		value       interface{}
		function    func(interface{}) interface{}
		expected    interface{}
		shouldPanic bool
	}{
		{
			title: "fromString string value",
			value: "uno",
			function: func(value interface{}) interface{} {
				strValue, _ := value.(string)
				return FromString(strValue)
			},
			expected: IntOrString("uno"),
		},
		{
			title: "fromString numeric value",
			value: "1",
			function: func(value interface{}) interface{} {
				strValue, _ := value.(string)
				return FromString(strValue)
			},
			expected: IntOrString("1"),
		},
		{
			title: "fromInt32",
			value: int32(1),
			function: func(value interface{}) interface{} {
				int32Value, _ := value.(int32)
				return FromInt32(int32Value)
			},
			expected: IntOrString("1"),
		},
		{
			title: "Int32",
			value: IntOrString("1"),
			function: func(value interface{}) interface{} {
				intOrStrValue, _ := value.(IntOrString)
				return intOrStrValue.Int32()
			},
			expected: int32(1),
		},
		{
			title: "Int32 overflow",
			value: IntOrString("9223372036854775807"),
			function: func(value interface{}) interface{} {
				intOrStrValue, _ := value.(IntOrString)
				return intOrStrValue.Int32()
			},
			expected:    nil,
			shouldPanic: true,
		},
		{
			title: "Int32 form string value",
			value: IntOrString("uno"),
			function: func(value interface{}) interface{} {
				intOrStrValue, _ := value.(IntOrString)
				return intOrStrValue.Int32()
			},
			expected:    nil,
			shouldPanic: true,
		},
		{
			title: "Int32 form nul value",
			value: IntOrString(""),
			function: func(value interface{}) interface{} {
				intOrStrValue, _ := value.(IntOrString)
				return intOrStrValue.Int32()
			},
			expected:    nil,
			shouldPanic: true,
		},
		{
			title: "String form string",
			value: IntOrString("uno"),
			function: func(value interface{}) interface{} {
				intOrStrValue, _ := value.(IntOrString)
				return intOrStrValue.Str()
			},
			expected:    "uno",
			shouldPanic: false,
		},
		{
			title: "String form nul value",
			value: IntOrString(""),
			function: func(value interface{}) interface{} {
				intOrStrValue, _ := value.(IntOrString)
				return intOrStrValue.Str()
			},
			expected:    "",
			shouldPanic: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			defer func() {
				panicked := recover()
				if panicked != nil && !tc.shouldPanic {
					t.Fatalf("panicked %v", panicked)
				}
			}()

			// if this conversion panics, the defer function checks if this is normal
			value := tc.function(tc.value)

			if tc.shouldPanic {
				t.Fatal("should had panicked")
			}

			// if conversion should panic expected value is undefined, so don't assert it
			if value != tc.expected {
				t.Fatalf("expected %s got %s", tc.expected, value)
			}
		})
	}
}

func Test_AsPercentage(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		value       IntOrString
		expect      int32
		expectError bool
	}{
		{
			title:       "percentage value",
			value:       IntOrString("10%"),
			expect:      10,
			expectError: false,
		},
		{
			title:       "float percentage value",
			value:       IntOrString("10.5%"),
			expect:      10,
			expectError: true,
		},
		{
			title:       "non int percentage value",
			value:       IntOrString("foo%"),
			expect:      10,
			expectError: true,
		},
		{
			title:       "int value",
			value:       IntOrString("10"),
			expect:      0,
			expectError: true,
		},
		{
			title:       "invalid value",
			value:       IntOrString("foo"),
			expect:      0,
			expectError: true,
		},
		{
			title:       "only percentage",
			value:       IntOrString("%"),
			expect:      0,
			expectError: true,
		},
		{
			title:       "extra percentage",
			value:       IntOrString("10%%"),
			expect:      0,
			expectError: true,
		},
		{
			title:       "leading percentage",
			value:       IntOrString("%10"),
			expect:      0,
			expectError: true,
		},
		{
			title:       "nul value",
			value:       NullValue,
			expect:      0,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			value, ok := tc.value.AsPercentage()

			if !ok && tc.expectError {
				return
			}

			if !ok && !tc.expectError {
				t.Fatal("failed")
			}

			if ok && tc.expectError {
				t.Fatalf("should have failed")
			}

			if value != tc.expect {
				t.Fatalf("expected %d got %d", tc.expect, value)
			}
		})
	}
}
