package api

import (
	"testing"
)

func Test_Validations(t *testing.T) {
	t.Parallel()
	type prototype struct {
		FieldString string
		FieldInt    int64
		FieldFloat  float64
		FieldStruct struct {
			SubfieldInt    int
			SubfieldStruct struct {
				SubSubfieldString string
			}
		}
		FieldMap   map[string]string
		FieldArray []string
	}

	testCases := []struct {
		description string
		actual      interface{}
		expected    interface{}
		expectError bool
	}{
		{
			description: "Valid mapping",
			expected:    prototype{},
			actual: map[string]interface{}{
				"fieldString": "string",
				"fieldInt":    int64(1),
				"fieldFloat":  float64(1),
				"fieldStruct": map[string]interface{}{
					"subfieldInt": int64(0),
					"subfieldStruct": map[string]interface{}{
						"subSubfieldString": "string",
					},
				},
				"fieldArray": []interface{}{},
			},
			expectError: false,
		},
		{
			description: "Valid mapping with snake_case",
			expected:    prototype{},
			actual: map[string]interface{}{
				"field_string": "string",
				"field_int":    int64(1),
				"field_float":  float64(1),
				"field_struct": map[string]interface{}{
					"subfield_int": int64(0),
					"subfield_struct": map[string]interface{}{
						"sub_subfield_string": "string",
					},
				},
				"field_array": []interface{}{},
			},
			expectError: false,
		},
		{
			description: "Valid mapping int to float",
			expected:    prototype{},
			actual: map[string]interface{}{
				"field_string": "string",
				"field_int":    int64(1),
				"fieldFloat":   int64(1),
				"field_struct": map[string]interface{}{
					"subfield_int": int64(0),
					"subfield_struct": map[string]interface{}{
						"sub_subfield_string": "string",
					},
				},
				"field_array": []interface{}{},
			},
			expectError: false,
		},
		{
			description: "Valid mapping float to int",
			expected:    prototype{},
			actual: map[string]interface{}{
				"field_string": "string",
				"field_int":    float64(1),
				"fieldFloat":   float64(1),
				"field_struct": map[string]interface{}{
					"subfield_int": int64(0),
					"subfield_struct": map[string]interface{}{
						"sub_subfield_string": "string",
					},
				},
				"field_array": []interface{}{},
			},
			expectError: false,
		},
		{
			description: "Invalid mapping (unknown field)",
			expected:    prototype{},
			actual: map[string]interface{}{
				"unknownField": "string",
				"fieldString":  "string",
				"fieldInt":     int64(1),
				"fieldFloat":   float64(1),
				"fieldStruct": map[string]interface{}{
					"subfieldInt": int64(0),
					"subfieldStruct": map[string]interface{}{
						"subSubfieldString": "string",
					},
				},
				"fieldArray": []interface{}{},
			},
			expectError: true,
		},
		{
			description: "Invalid mapping (string to int)",
			expected:    prototype{},
			actual: map[string]interface{}{
				"fieldString": "string",
				"fieldInt":    "1",
				"fieldFloat":  float64(1),
				"fieldStruct": map[string]interface{}{
					"subfieldInt": int64(0),
					"subfieldStruct": map[string]interface{}{
						"subSubfieldString": "string",
					},
				},
				"fieldArray": []interface{}{},
			},
			expectError: true,
		},
		{
			description: "Invalid mapping (int to string)",
			expected:    prototype{},
			actual: map[string]interface{}{
				"fieldString": int64(1),
				"fieldInt":    int64(1),
				"fieldFloat":  float64(1),
				"fieldStruct": map[string]interface{}{
					"subfieldInt": int64(0),
					"subfieldStruct": map[string]interface{}{
						"subSubfieldString": "string",
					},
				},
				"fieldArray": []interface{}{},
			},
			expectError: true,
		},
		{
			description: "Invalid mapping (struct to string)",
			expected:    prototype{},
			actual: map[string]interface{}{
				"fieldString": struct{}{},
				"fieldInt":    int64(1),
				"fieldFloat":  float64(1),
				"fieldStruct": map[string]interface{}{
					"subfieldInt": int64(0),
					"subfieldStruct": map[string]interface{}{
						"subSubfieldString": "string",
					},
				},
				"fieldArray": []interface{}{},
			},
			expectError: true,
		},
		{
			description: "Invalid mapping (string to struct)",
			expected:    prototype{},
			actual:      "string",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			err := ValidateStruct(tc.actual, tc.expected)
			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
			}
		})
	}
}
