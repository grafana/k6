/**
 * Copyright (c) Microsoft Corporation.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package common

import (
	"errors"
	"regexp"
	"strings"
)

// TODO: Reimplement this by looking at:
// https://github.com/microsoft/playwright/blob/
// 		35cd6fb279ccf4c91eae4b8094697eceb61426cc/
// 		packages/playwright-core/src/utils/isomorphic/selectorParser.ts#L154
// Current implementation is a simplified version of the above and cannot handle all edge cases.

// Matches `name:body`, a query engine name and selector for that engine.
var reQueryEngine *regexp.Regexp = regexp.MustCompile(`^[a-zA-Z_0-9-+:*]+$`)

// Matches start of XPath query.
var reXPathSelector *regexp.Regexp = regexp.MustCompile(`^\(*//`)

type SelectorPart struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

type Selector struct {
	Selector string          `json:"selector"`
	Parts    []*SelectorPart `json:"parts"`

	// By default chained queries resolve to elements matched by the last selector,
	// but a selector can be prefixed with `*` to capture elements resolved by
	// an intermediate selector.
	Capture *int `json:"capture"`
}

func NewSelector(selector string) (*Selector, error) {
	s := Selector{
		Selector: selector,
		Parts:    make([]*SelectorPart, 0, 1),
		Capture:  nil,
	}
	err := s.parse()
	return &s, err
}

func (s *Selector) appendPart(p *SelectorPart, capture bool) error {
	s.Parts = append(s.Parts, p)
	if capture {
		if s.Capture != nil {
			return errors.New("only one of the selectors can capture using * modifier")
		}
		s.Capture = new(int)
		*s.Capture = (len(s.Parts) - 1)
	}
	return nil
}

//nolint:cyclop,funlen
func (s *Selector) parse() error {
	parsePart := func(selector string, start, index int) (*SelectorPart, bool) {
		part := strings.TrimSpace(selector[start:index])
		eqIndex := strings.Index(part, "=")
		var name, body string

		switch {
		// Using TrimQuote prevents issues with selectors, allowing us to
		// handle quoted text with internal selectors, such as:
		//
		//       page.getByRole("listitem >> internal:has-text='Product 2'")
		//
		// We'd receive the following error because of the quotes:
		//
		//       Uncaught (in promise) getting text content of
		//       "internal:role=listitem >> internal:has-text='Product 2'
		//       >> nth=0": SyntaxError: Unexpected token ''', "'Product 2'"
		//       is not valid JSON
		//
		// Selectors, such as, internal:has-text, by their nature, are
		// exposed to users, even if we don't document their use.
		case strings.HasPrefix(part, "internal:has-text="):
			name = "internal:has-text"
			body = TrimQuotes(part[eqIndex+1:])
		case strings.HasPrefix(part, "internal:has-not-text="):
			name = "internal:has-not-text"
			body = TrimQuotes(part[eqIndex+1:])
		case strings.HasPrefix(part, "nth="):
			name = "nth"
			body = part[eqIndex+1:]
		case eqIndex != -1 && reQueryEngine.Match([]byte(strings.TrimSpace(part[0:eqIndex]))):
			name = strings.TrimSpace(part[0:eqIndex])
			body = part[eqIndex+1:]
		case len(part) > 1 && part[0] == '"' && part[len(part)-1] == '"':
			name = "text"
			body = part
		case isQuotedText(part):
			name = "text"
			body = part
		case reXPathSelector.Match([]byte(part)) || strings.HasPrefix(part, ".."):
			// If selector starts with '//' or '//' prefixed with multiple opening
			// parenthesis, consider xpath. @see https://github.com/microsoft/playwright/issues/817
			// If selector starts with '..', consider xpath as well.
			name = "xpath"
			body = part
		case strings.HasPrefix(part, "internal:role="):
			name = "internal:role"
			body = part
		case strings.HasPrefix(part, "internal:attr="):
			name = "internal:attr"
			body = part
		case strings.HasPrefix(part, "internal:label="):
			name = "internal:label"
			body = part
		case strings.HasPrefix(part, "internal:text="):
			name = "internal:text"
			body = part
		case strings.HasPrefix(part, "internal:control="):
			name = "internal:control"
			body = part[eqIndex+1:]
		default:
			name = "css"
			body = part
		}

		capture := false
		if name[0] == '*' {
			capture = true
			name = name[1:]
		}

		return &SelectorPart{Name: name, Body: body}, capture
	}

	if !strings.Contains(s.Selector, ">>") {
		part, capture := parsePart(s.Selector, 0, len(s.Selector))
		err := s.appendPart(part, capture)
		if err != nil {
			return err
		}
		return nil
	}

	start := 0
	index := 0
	var quote byte

	for index < len(s.Selector) {
		c := s.Selector[index]
		switch {
		case c == '\\' && index+1 < len(s.Selector):
			index += 2
		case c == quote:
			quote = byte(0)
			index++
		case quote == 0 && (c == '"' || c == '\'' || c == '`'):
			quote = c
			index++
		case quote == 0 && c == '>' && s.Selector[index+1] == '>':
			part, capture := parsePart(s.Selector, start, index)
			err := s.appendPart(part, capture)
			if err != nil {
				return err
			}
			index += 2
			start = index
		default:
			index++
		}
	}

	part, capture := parsePart(s.Selector, start, index)
	err := s.appendPart(part, capture)
	if err != nil {
		return err
	}
	return nil
}
