package common

import (
	"errors"
	"reflect"
	"testing"
)

func partsToValues(parts []*SelectorPart) []SelectorPart {
	out := make([]SelectorPart, len(parts))
	for i, p := range parts {
		out[i] = *p
	}
	return out
}

func TestSelectorParse(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		input         string
		expectedParts []*SelectorPart
		expectedCap   *int
		expectedError error
	}{
		{
			name:          "Empty selector",
			input:         "",
			expectedParts: nil,
			expectedCap:   nil,
			expectedError: ErrEmptySelector,
		},
		{
			name:  "Escaped quotes in text",
			input: `text="Click the \"Submit\" button"`,
			expectedParts: []*SelectorPart{
				{Name: "text", Body: `"Click the \"Submit\" button"`},
			},
			expectedCap: nil,
		},
		{
			name:  "Chain with >> and nth",
			input: `css=div.container >> text="Login" >> nth=0`,
			expectedParts: []*SelectorPart{
				{Name: "css", Body: "div.container"},
				{Name: "text", Body: `"Login"`},
				{Name: "nth", Body: "0"},
			},
			expectedCap: nil,
		},
		{
			name:  "XPath selector",
			input: `(//div[@class="item"])[2] >> text="Buy"`,
			expectedParts: []*SelectorPart{
				{Name: "xpath", Body: `(//div[@class="item"])[2]`},
				{Name: "text", Body: `"Buy"`},
			},
			expectedCap: nil,
		},
		{
			name:  "Capture selector",
			input: `*css=div.list >> text="Product 2"`,
			expectedParts: []*SelectorPart{
				{Name: "css", Body: "div.list"},
				{Name: "text", Body: `"Product 2"`},
			},
			expectedCap: func() *int { i := 0; return &i }(),
		},
		{
			name:  "Internal has-text",
			input: `internal:has-text="Some text"`,
			expectedParts: []*SelectorPart{
				{Name: "internal:has-text", Body: `"Some text"`},
			},
			expectedCap: nil,
		},
		{
			name:  "Capture + escaped quotes",
			input: `*text="Item \"Special\" >> Button"`,
			expectedParts: []*SelectorPart{
				{Name: "text", Body: `"Item \"Special\" >> Button"`},
			},
			expectedCap: func() *int { i := 0; return &i }(),
		},
		{
			name:  "Spaces around =",
			input: `text = "Login"`,
			expectedParts: []*SelectorPart{
				{Name: "text", Body: ` "Login"`},
			},
			expectedCap: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sel, err := NewSelector(tc.input)

			if tc.expectedError != nil {
				if err == nil {
					t.Fatalf("Expected error: %v, got nil", tc.expectedError)
				}

				if !errors.Is(err, tc.expectedError) {
					t.Fatalf("Unexpected error: %v", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !reflect.DeepEqual(sel.Parts, tc.expectedParts) {
				t.Errorf("Parts mismatch.\nGot: %#v\nExpected: %#v", partsToValues(sel.Parts), partsToValues(tc.expectedParts))
			}

			if (sel.Capture == nil) != (tc.expectedCap == nil) ||
				(sel.Capture != nil && tc.expectedCap != nil && *sel.Capture != *tc.expectedCap) {
				t.Errorf("Capture mismatch.\nGot: %#v\nExpected: %#v", sel.Capture, tc.expectedCap)
			}
		})
	}
}
