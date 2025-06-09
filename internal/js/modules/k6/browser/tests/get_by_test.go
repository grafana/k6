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

	// This test all the explicit roles that are valid for the role based
	// selector engine that is in the injectd_script.js file. Explicit roles
	// are roles that are explicitly defined in the HTML using the correct
	// role attribute.
	t.Run("explicit", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())
		p := tb.NewPage(nil)
		opts := &common.FrameGotoOptions{
			Timeout: common.DefaultTimeout,
		}
		_, err := p.Goto(
			tb.staticURL("get_by_role_explicit.html"),
			opts,
		)
		require.NoError(t, err)

		tests := []struct {
			role         string
			expected     int
			expectedText string
		}{
			{role: "alert", expected: 1, expectedText: "Alert"},
			{role: "alertdialog", expected: 1, expectedText: "Alert Dialog"},
			{role: "application", expected: 1, expectedText: "Application"},
			{role: "article", expected: 1, expectedText: "Article"},
			{role: "banner", expected: 1, expectedText: "Banner"},
			{role: "blockquote", expected: 1, expectedText: "Blockquote"},
			{role: "button", expected: 1, expectedText: "Button"},
			{role: "caption", expected: 1, expectedText: "Caption"},
			{role: "cell", expected: 1, expectedText: "Cell"},
			{role: "checkbox", expected: 1, expectedText: "Checkbox"},
			{role: "code", expected: 1, expectedText: "Code"},
			{role: "columnheader", expected: 1, expectedText: "Column Header"},
			{role: "combobox", expected: 1, expectedText: "Combobox"},
			{role: "complementary", expected: 1, expectedText: "Complementary"},
			{role: "contentinfo", expected: 1, expectedText: "Content Info"},
			{role: "definition", expected: 1, expectedText: "Definition"},
			{role: "deletion", expected: 1, expectedText: "Deletion"},
			{role: "dialog", expected: 1, expectedText: "Dialog"},
			{role: "directory", expected: 1, expectedText: "Directory"},
			// The original document plus the one within the html <section>
			{role: "document", expected: 2, expectedText: ""},
			{role: "emphasis", expected: 1, expectedText: "Emphasis"},
			{role: "feed", expected: 1, expectedText: "Feed"},
			{role: "figure", expected: 1, expectedText: "Figure"},
			{role: "form", expected: 1, expectedText: "Form"},
			{role: "generic", expected: 1, expectedText: "Generic"},
			{role: "grid", expected: 1, expectedText: "Grid"},
			{role: "gridcell", expected: 1, expectedText: "Grid Cell"},
			{role: "group", expected: 1, expectedText: "Group"},
			{role: "heading", expected: 1, expectedText: "Heading"},
			{role: "img", expected: 1, expectedText: "Image"},
			{role: "insertion", expected: 1, expectedText: "Insertion"},
			{role: "link", expected: 1, expectedText: "Link"},
			{role: "list", expected: 1, expectedText: "List"},
			{role: "listbox", expected: 1, expectedText: "List Box"},
			{role: "listitem", expected: 1, expectedText: "List Item"},
			{role: "log", expected: 1, expectedText: "Log"},
			{role: "main", expected: 1, expectedText: "Main"},
			{role: "mark", expected: 1, expectedText: "Mark"},
			{role: "marquee", expected: 1, expectedText: "Marquee"},
			{role: "math", expected: 1, expectedText: "Math"},
			{role: "meter", expected: 1, expectedText: "Meter"},
			{role: "menu", expected: 1, expectedText: "Menu"},
			{role: "menubar", expected: 1, expectedText: "Menu Bar"},
			{role: "menuitem", expected: 1, expectedText: "Menu Item"},
			{role: "menuitemcheckbox", expected: 1, expectedText: "Menu Item Checkbox"},
			{role: "menuitemradio", expected: 1, expectedText: "Menu Item Radio"},
			{role: "navigation", expected: 1, expectedText: "Navigation"},
			{role: "note", expected: 1, expectedText: "Note"},
			{role: "none", expected: 1, expectedText: "None"},
			{role: "option", expected: 1, expectedText: "Option"},
			{role: "paragraph", expected: 1, expectedText: "Paragraph"},
			{role: "presentation", expected: 1, expectedText: "Presentation"},
			{role: "progressbar", expected: 1, expectedText: "Progress Bar"},
			{role: "radio", expected: 1, expectedText: "Radio"},
			{role: "radiogroup", expected: 1, expectedText: "Radio Group"},
			{role: "region", expected: 1, expectedText: "Region"},
			{role: "row", expected: 1, expectedText: "Row"},
			{role: "rowgroup", expected: 1, expectedText: "Row Group"},
			{role: "rowheader", expected: 1, expectedText: "Row Header"},
			{role: "scrollbar", expected: 1, expectedText: "Scroll Bar"},
			{role: "search", expected: 1, expectedText: "Search"},
			{role: "searchbox", expected: 1, expectedText: "Search Box"},
			{role: "separator", expected: 1, expectedText: "Separator"},
			{role: "slider", expected: 1, expectedText: "Slider"},
			{role: "spinbutton", expected: 1, expectedText: "Spin Button"},
			{role: "strong", expected: 1, expectedText: "Strong"},
			{role: "subscript", expected: 1, expectedText: "Subscript"},
			{role: "superscript", expected: 1, expectedText: "Superscript"},
			{role: "status", expected: 1, expectedText: "Status"},
			{role: "switch", expected: 1, expectedText: "Switch"},
			{role: "tab", expected: 1, expectedText: "Tab"},
			{role: "tablist", expected: 1, expectedText: "Tab List"},
			{role: "tabpanel", expected: 1, expectedText: "Tab Panel"},
			{role: "table", expected: 1, expectedText: "Table"},
			{role: "term", expected: 1, expectedText: "Term"},
			{role: "textbox", expected: 1, expectedText: "Text Box"},
			{role: "time", expected: 1, expectedText: "Time"},
			{role: "timer", expected: 1, expectedText: "Timer"},
			{role: "toolbar", expected: 1, expectedText: "Toolbar"},
			{role: "tooltip", expected: 1, expectedText: "Tooltip"},
			{role: "tree", expected: 1, expectedText: "Tree"},
			{role: "treegrid", expected: 1, expectedText: "Tree Grid"},
			{role: "treeitem", expected: 1, expectedText: "Tree Item"},
		}
		for _, tt := range tests {
			t.Run(tt.role, func(t *testing.T) {
				t.Parallel()

				l := p.GetByRole(tt.role, nil)
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
