package tests

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/js/modules/k6/browser/common"
)

func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func int64Ptr(i int64) *int64 {
	return &i
}

func TestGetByRoleSuccess(t *testing.T) {
	t.Parallel()

	// This tests all the options, and different attributes (such as explicit
	// aria attributes vs the text value of an element) that can be used in
	// the DOM with the same role.
	t.Run("edge_cases", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())
		p := tb.NewPage(nil)
		opts := &common.FrameGotoOptions{
			Timeout: common.DefaultTimeout,
		}
		_, err := p.Goto(
			tb.staticURL("get_by_role_edge_cases.html"),
			opts,
		)
		require.NoError(t, err)

		tests := []struct {
			name         string
			role         string
			opts         *common.GetByRoleOptions
			expected     int
			expectedText string
		}{
			{
				name:     "text_content_as_name",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: stringPtr(`"Submit Form"`)},
				expected: 1, expectedText: "",
			},
			{
				name:     "aria_label_as_name",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: stringPtr(`"Save Draft"`)},
				expected: 1, expectedText: "",
			},
			{
				name:     "aria_labelledby_as_name",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: stringPtr(`"Upload"`)},
				expected: 1, expectedText: "labelledby-upload-button",
			},
			{
				name:     "hidden_text_nodes_should_be_ignored",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: stringPtr(`"FooBar"`)},
				expected: 0, expectedText: "",
			},
			{
				name:     "only_visible_node",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: stringPtr(`"Bar"`)},
				expected: 1, expectedText: "Bar",
			},
			{
				name:     "regex_matching",
				role:     "heading",
				opts:     &common.GetByRoleOptions{Name: stringPtr(`/^[a-z0-9]+$/`)},
				expected: 1, expectedText: "abc123",
			},
			{
				name:     "selected_option",
				role:     "option",
				opts:     &common.GetByRoleOptions{Selected: boolPtr(true)},
				expected: 1, expectedText: "One",
			},
			{
				name:     "pressed_option",
				role:     "button",
				opts:     &common.GetByRoleOptions{Pressed: boolPtr(true)},
				expected: 1, expectedText: "Toggle",
			},
			{
				name:     "expanded_option",
				role:     "button",
				opts:     &common.GetByRoleOptions{Expanded: boolPtr(true)},
				expected: 1, expectedText: "Expanded",
			},
			{
				name:     "level_option",
				role:     "heading",
				opts:     &common.GetByRoleOptions{Level: int64Ptr(6)},
				expected: 1, expectedText: "Section",
			},
			{
				name:     "checked_option",
				role:     "checkbox",
				opts:     &common.GetByRoleOptions{Checked: boolPtr(true)},
				expected: 1, expectedText: "",
			},
			{
				name:     "radio_checked_option",
				role:     "radio",
				opts:     &common.GetByRoleOptions{Checked: boolPtr(true)},
				expected: 1, expectedText: "",
			},
			{
				name:     "disabled_option",
				role:     "button",
				opts:     &common.GetByRoleOptions{Disabled: boolPtr(true)},
				expected: 1, expectedText: "Go",
			},
			{
				name:     "include_css_hidden",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: stringPtr(`"Hidden X Button"`), IncludeHidden: boolPtr(true)},
				expected: 1, expectedText: "X",
			},
			{
				name:     "include_aria_hidden",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: stringPtr(`"Hidden Hi Button"`), IncludeHidden: boolPtr(true)},
				expected: 1, expectedText: "Hi",
			},
			{
				name:     "combo_options",
				role:     "button",
				opts:     &common.GetByRoleOptions{Pressed: boolPtr(false), Name: stringPtr(`"Archive"`), IncludeHidden: boolPtr(true)},
				expected: 1, expectedText: "Combo Options Button",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				l := p.GetByRole(tt.role, tt.opts)
				c, err := l.Count()
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, c)

				if tt.expectedText != "" {
					text, err := l.InnerText(sobek.Undefined())
					assert.NoError(t, err)
					assert.Equal(t, tt.expectedText, text)
				}
			})
		}
	})
}

func TestGetByRoleFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		role          string
		opts          *common.GetByRoleOptions
		expectedError string
	}{
		{
			"missing_quotes_on_string",
			"button",
			&common.GetByRoleOptions{Name: stringPtr(`Submit Form`)},
			"InvalidSelectorError: Error while parsing selector `button[name=Submit Form]`",
		},
		{
			"missing_role",
			"",
			nil,
			"counting elements: InvalidSelectorError: Error while parsing selector `` - selector cannot be empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)
			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err := p.Goto(
				tb.staticURL("get_by_role.html"),
				opts,
			)
			require.NoError(t, err)

			l := p.GetByRole(tt.role, tt.opts)
			_, err = l.Count()
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}
