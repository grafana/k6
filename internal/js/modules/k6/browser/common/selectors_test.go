package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectorParse(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		input         string
		expectedParts []*SelectorPart
		expectedCap   *int
		expectedError string
	}{
		{
			name:          "Empty selector",
			input:         "",
			expectedParts: nil,
			expectedCap:   nil,
			expectedError: "provided selector is empty",
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
		{
			name:  "Space between >>",
			input: `css="div" >> >> text="item"`,
			expectedParts: []*SelectorPart{
				{Name: "css", Body: `"div"`},
				{Name: "text", Body: `"item"`},
			},
			expectedCap: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sel, err := NewSelector(tc.input)

			if tc.expectedError != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedParts, sel.Parts, "Parts mismatch")
			assert.EqualValues(t, tc.expectedCap, sel.Capture, "Capture mismatch")
		})
	}
}

func TestSelectorParseInternalOrQuotedJSON(t *testing.T) {
	t.Parallel()

	parsed, err := NewSelector("a")
	require.NoError(t, err)
	encoded, err := json.Marshal(parsed)
	require.NoError(t, err)
	quoted, err := json.Marshal(string(encoded))
	require.NoError(t, err)
	input := "button >> internal:or=" + string(quoted)

	sel, err := NewSelector(input)
	require.NoError(t, err)

	assert.Equal(t, []*SelectorPart{
		{Name: "css", Body: "button"},
		{Name: "internal:or", Body: string(quoted)},
	}, sel.Parts)
}

// TestWithControlCharInSelector checks wire encoding with control characters, ensuring it will decode correctly in JS.
func TestWithControlCharInSelector(t *testing.T) {
	t.Parallel()

	parsed, err := NewSelector("text=\u007f")
	require.NoError(t, err)
	encoded, err := json.Marshal(parsed)
	require.NoError(t, err)

	// Must match Locator.Or's outer encoding.
	quoted, err := json.Marshal(string(encoded))
	require.NoError(t, err)

	got, err := decodeInternalOrBody(string(quoted))
	require.NoError(t, err)
	require.Len(t, got.Parts, 1)
	assert.Equal(t, "text", got.Parts[0].Name)
	assert.Equal(t, "\u007f", got.Parts[0].Body)
}

// decodeInternalOrBody mirrors the injected script's
// JSON.parse(JSON.parse(part.body)) for internal:or.
func decodeInternalOrBody(body string) (*Selector, error) {
	var once string
	if err := json.Unmarshal([]byte(body), &once); err != nil {
		return nil, err
	}
	var sel Selector
	if err := json.Unmarshal([]byte(once), &sel); err != nil {
		return nil, err
	}
	return &sel, nil
}
