// practically none of this work on windows
//go:build !windows

package tests

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/js/modules/k6/browser/common"
)

func TestGetByRoleSuccess(t *testing.T) {
	t.Parallel()

	// This test all the implicit roles that are valid for the role based
	// selector engine that is in the injectd_script.js file. Implicit roles
	// are roles that are not explicitly defined in the HTML, but are
	// implied by the context of the element.
	t.Run("implicit", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())
		p := tb.NewPage(nil)
		opts := &common.FrameGotoOptions{
			Timeout: common.DefaultTimeout,
		}
		_, err := p.Goto(
			tb.staticURL("get_by_role_implicit.html"),
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
				name:     "link",
				role:     "link",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'Link text'`)},
				expected: 1, expectedText: "Link text",
			},
			{
				name:     "area",
				role:     "link",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'Map area'`)},
				expected: 1, expectedText: "",
			},
			{
				name:     "button",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'Click'`)},
				expected: 1, expectedText: "Click",
			},
			{
				name:     "submit_type",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'Submit'`)},
				expected: 1, expectedText: "",
			},
			{
				name:     "image_type",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'Image Button'`)},
				expected: 1, expectedText: "",
			},
			{
				name:     "checkbox_type",
				role:     "checkbox",
				expected: 1, expectedText: "",
			},
			{
				name:     "radio_type",
				role:     "radio",
				expected: 1, expectedText: "",
			},
			{
				name:     "text_type",
				role:     "textbox",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'Text type'`)},
				expected: 1, expectedText: "",
			},
			{
				name:     "textarea",
				role:     "textbox",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'Text area'`)},
				expected: 1, expectedText: "Textarea",
			},
			{
				name:     "search_type",
				role:     "searchbox",
				expected: 1, expectedText: "",
			},
			{
				name:     "range_type",
				role:     "slider",
				expected: 1, expectedText: "",
			},
			{
				name:     "number_type",
				role:     "spinbutton",
				expected: 1, expectedText: "",
			},
			{
				name:     "progress",
				role:     "progressbar",
				expected: 1, expectedText: "Progress",
			},
			{
				name:     "output",
				role:     "status",
				expected: 1, expectedText: "Output",
			},
			{
				name:     "details_summary",
				role:     "group",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'details'`)},
				expected: 1, expectedText: "SummaryDetails",
			},
			{
				name:     "dialog",
				role:     "dialog",
				expected: 1, expectedText: "Dialog",
			},
			{
				name:     "h1",
				role:     "heading",
				opts:     &common.GetByRoleOptions{Level: toPtr(int64(1))},
				expected: 1, expectedText: "Heading1",
			},
			{
				name:     "h2",
				role:     "heading",
				opts:     &common.GetByRoleOptions{Level: toPtr(int64(2))},
				expected: 1, expectedText: "Heading2",
			},
			{
				name:     "h3",
				role:     "heading",
				opts:     &common.GetByRoleOptions{Level: toPtr(int64(3))},
				expected: 1, expectedText: "Heading3",
			},
			{
				name:     "h4",
				role:     "heading",
				opts:     &common.GetByRoleOptions{Level: toPtr(int64(4))},
				expected: 1, expectedText: "Heading4",
			},
			{
				name:     "h5",
				role:     "heading",
				opts:     &common.GetByRoleOptions{Level: toPtr(int64(5))},
				expected: 1, expectedText: "Heading5",
			},
			{
				name:     "h6",
				role:     "heading",
				opts:     &common.GetByRoleOptions{Level: toPtr(int64(6))},
				expected: 1, expectedText: "Heading6",
			},
			{
				name:     "hr",
				role:     "separator",
				expected: 1, expectedText: "",
			},
			{
				name:     "img",
				role:     "img",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'Img'`)},
				expected: 1, expectedText: "",
			},
			{
				name:     "img_presentation",
				role:     "presentation",
				expected: 1, expectedText: "",
			},
			{
				name:     "ul_list",
				role:     "list",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'ul'`)},
				expected: 1, expectedText: "",
			},
			{
				name:     "ol_list",
				role:     "list",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'ol'`)},
				expected: 1, expectedText: "",
			},
			{
				name:     "ul_li_listitem",
				role:     "listitem",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'ul-li'`)},
				expected: 1, expectedText: "Item1",
			},
			{
				name:     "ol_li_listitem",
				role:     "listitem",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'ol-li'`)},
				expected: 1, expectedText: "Item2",
			},
			{
				name:     "dd",
				role:     "definition",
				expected: 1, expectedText: "Description",
			},
			{
				name:     "dt_dfn",
				role:     "term",
				expected: 2, expectedText: "",
			},
			{
				name:     "fieldset_legend",
				role:     "group",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'Legend'`)},
				expected: 1, expectedText: "Legend",
			},
			{
				name:     "figure_figcaption",
				role:     "figure",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'Caption'`)},
				expected: 1, expectedText: "Caption",
			},
			{
				name:     "table",
				role:     "table",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'table1'`)},
				expected: 1, expectedText: "",
			},
			{
				name:     "table_scope_row",
				role:     "rowheader",
				expected: 1, expectedText: "Head Row",
			},
			{
				name:     "table_scope_col",
				role:     "columnheader",
				expected: 1, expectedText: "Head Column",
			},
			{
				name:     "table_head_cell",
				role:     "cell",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'th'`)},
				expected: 1, expectedText: "Head Cell",
			},
			{
				name:     "table_head_gridcell",
				role:     "gridcell",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'th gridcell'`)},
				expected: 1, expectedText: "Head Gridcell",
			},
			{
				name:     "table_body",
				role:     "rowgroup",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'tbody'`)},
				expected: 1, expectedText: "Cell",
			},
			{
				name:     "table_foot",
				role:     "rowgroup",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'tfoot'`)},
				expected: 1, expectedText: "Foot",
			},
			{
				name:     "table_tr",
				role:     "row",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'tr'`)},
				expected: 1, expectedText: "Row",
			},
			{
				name:     "table_td_cell",
				role:     "cell",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'td'`)},
				expected: 1, expectedText: "Column Cell",
			},
			{
				name:     "table_td_gridcell",
				role:     "gridcell",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'td gridcell'`)},
				expected: 1, expectedText: "Column Gridcell",
			},
			{
				name:     "main",
				role:     "main",
				expected: 1, expectedText: "Main",
			},
			{
				name:     "nav",
				role:     "navigation",
				expected: 1, expectedText: "Nav",
			},
			{
				name:     "article",
				role:     "article",
				expected: 1, expectedText: "Article",
			},
			{
				name:     "aside",
				role:     "complementary",
				expected: 1, expectedText: "Aside",
			},
			// Only works when outside the <section> element.
			{
				name:     "header",
				role:     "banner",
				expected: 1, expectedText: "Header",
			},
			// Only works when outside the <section> element.
			{
				name:     "footer",
				role:     "contentinfo",
				expected: 1, expectedText: "Footer",
			},
			// Only works with aria labels.
			{
				name:     "form",
				role:     "form",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'form'`)},
				expected: 1, expectedText: "",
			},
			// Only works with aria labels.
			{
				name:     "section",
				role:     "region",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'Region Section'`)},
				expected: 1, expectedText: "Region content",
			},
			{
				name:     "blockquote",
				role:     "blockquote",
				expected: 1, expectedText: "Blockquote text",
			},
			{
				name:     "caption",
				role:     "caption",
				expected: 1, expectedText: "Table Caption",
			},
			{
				name:     "code",
				role:     "code",
				expected: 1, expectedText: "Code sample",
			},
			{
				name:     "del",
				role:     "deletion",
				expected: 1, expectedText: "Deleted text",
			},
			{
				name:     "em",
				role:     "emphasis",
				expected: 1, expectedText: "Emphasized text",
			},
			{
				name:     "ins",
				role:     "insertion",
				expected: 1, expectedText: "Inserted text",
			},
			{
				name:     "mark",
				role:     "mark",
				expected: 1, expectedText: "Marked text",
			},
			{
				name:     "math",
				role:     "math",
				expected: 1, expectedText: "x=1",
			},
			{
				name:     "menu",
				role:     "list",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'menu'`)},
				expected: 1, expectedText: "Menu item",
			},
			{
				name:     "meter",
				role:     "meter",
				expected: 1, expectedText: "50%",
			},
			{
				name:     "p",
				role:     "paragraph",
				expected: 1, expectedText: "Paragraph text",
			},
			{
				name:     "strong",
				role:     "strong",
				expected: 1, expectedText: "Strong text",
			},
			{
				name:     "sub",
				role:     "subscript",
				expected: 1, expectedText: "Subscript",
			},
			{
				name:     "sup",
				role:     "superscript",
				expected: 1, expectedText: "Superscript",
			},
			{
				name:     "svg",
				role:     "img",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'svg'`)},
				expected: 1, expectedText: "",
			},
			{
				name:     "time",
				role:     "time",
				expected: 1, expectedText: "June 9, 2025",
			},
			{
				name:     "select",
				role:     "combobox",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'select'`)},
				expected: 1, expectedText: "",
			},
			{
				name:     "select_multiple",
				role:     "listbox",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'select multiple'`)},
				expected: 1, expectedText: "",
			},
			{
				name:     "datalist",
				role:     "listbox",
				expected: 1, expectedText: "",
			},
			// TODO: This is not working, not even in Playwright.
			// {
			// 	name:     "optgroup",
			// 	role:     "group",
			// 	opts:     &common.GetByRoleOptions{Name: toPtr(`'optgroup'`)},
			// 	expected: 1, expectedText: "",
			// },
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				l := p.GetByRole(tt.role, tt.opts)
				c, err := l.Count()
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, c)

				if tt.expectedText != "" {
					text, _, err := l.TextContent(sobek.Undefined())
					assert.NoError(t, err)
					assert.Equal(t, tt.expectedText, text)
				}
			})
		}
	})

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
				opts:     &common.GetByRoleOptions{Name: toPtr(`'Submit Form'`)},
				expected: 1, expectedText: "",
			},
			{
				name:     "not_exact",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'submit form'`), Exact: toPtr(false)},
				expected: 1, expectedText: "",
			},
			{
				name:     "exact_no_match",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'submit form'`), Exact: toPtr(true)},
				expected: 0, expectedText: "",
			},
			{
				name:     "exact_match",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'Submit Form'`), Exact: toPtr(true)},
				expected: 1, expectedText: "",
			},
			{
				name:     "aria_label_as_name",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'Save Draft'`)},
				expected: 1, expectedText: "",
			},
			{
				name:     "aria_labelledby_as_name",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'Upload'`)},
				expected: 1, expectedText: "labelledby-upload-button",
			},
			{
				name:     "hidden_text_nodes_should_be_ignored",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'FooBar'`)},
				expected: 0, expectedText: "",
			},
			{
				name:     "only_visible_node",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'Bar'`)},
				expected: 1, expectedText: "Bar",
			},
			{
				name:     "regex_matching",
				role:     "heading",
				opts:     &common.GetByRoleOptions{Name: toPtr(`/^[a-z0-9]+$/`)},
				expected: 1, expectedText: "abc123",
			},
			{
				name:     "selected_option",
				role:     "option",
				opts:     &common.GetByRoleOptions{Selected: toPtr(true)},
				expected: 1, expectedText: "One",
			},
			{
				name:     "pressed_option",
				role:     "button",
				opts:     &common.GetByRoleOptions{Pressed: toPtr(true)},
				expected: 1, expectedText: "Toggle",
			},
			{
				name:     "expanded_option",
				role:     "button",
				opts:     &common.GetByRoleOptions{Expanded: toPtr(true)},
				expected: 1, expectedText: "Expanded",
			},
			{
				name:     "level_option",
				role:     "heading",
				opts:     &common.GetByRoleOptions{Level: toPtr(int64(6))},
				expected: 1, expectedText: "Section",
			},
			{
				name:     "checked_option",
				role:     "checkbox",
				opts:     &common.GetByRoleOptions{Checked: toPtr(true)},
				expected: 1, expectedText: "",
			},
			{
				name:     "radio_checked_option",
				role:     "radio",
				opts:     &common.GetByRoleOptions{Checked: toPtr(true)},
				expected: 1, expectedText: "",
			},
			{
				name:     "disabled_option",
				role:     "button",
				opts:     &common.GetByRoleOptions{Disabled: toPtr(true)},
				expected: 1, expectedText: "Go",
			},
			{
				name:     "include_css_hidden",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'Hidden X Button'`), IncludeHidden: toPtr(true)},
				expected: 1, expectedText: "X",
			},
			{
				name:     "include_aria_hidden",
				role:     "button",
				opts:     &common.GetByRoleOptions{Name: toPtr(`'Hidden Hi Button'`), IncludeHidden: toPtr(true)},
				expected: 1, expectedText: "Hi",
			},
			{
				name:     "combo_options",
				role:     "button",
				opts:     &common.GetByRoleOptions{Pressed: toPtr(false), Name: toPtr(`'Archive'`), IncludeHidden: toPtr(true)},
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
			&common.GetByRoleOptions{Name: toPtr(`Submit Form`)},
			"Error while parsing selector `button[name=Submit Form]` - unexpected symbol",
		},
		{
			"missing_role",
			"",
			nil,
			"Error while parsing selector `` - selector cannot be empty",
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
