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

// Matches `name:body`, a query engine name and selector for that engine.
var reQueryEngine *regexp.Regexp = regexp.MustCompile(`^[a-zA-Z_0-9-+:*]+$`)

// Matches start of XPath query.
var reXPathSelector *regexp.Regexp = regexp.MustCompile(`^\(*//`)

// Matches the text selectors of the form text=<value>
var reTextSelector *regexp.Regexp = regexp.MustCompile(`^\s*text\s*=\s*(.*)$`)

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
	if selector == "" {
		return nil, errors.New("provided selector is empty")
	}

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

// parse splits the selector into parts, separated by `>>`, and identifies
// the query engine for each part.
//
//nolint:cyclop,funlen
func (s *Selector) parse() error {
	parsePart := func(part string) (*SelectorPart, bool) {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false
		}

		before, after, ok := strings.Cut(part, "=")
		var name, body string

		switch {
		case ok && reQueryEngine.MatchString(strings.TrimSpace(before)):
			name = strings.TrimSpace(before)
			body = after
		case len(part) > 1 && part[0] == '"' && part[len(part)-1] == '"':
			name = "text"
			body = part
		case len(part) > 1 && part[0] == '\'' && part[len(part)-1] == '\'':
			name = "text"
			body = part
		case reXPathSelector.MatchString(part) || strings.HasPrefix(part, ".."):
			name = "xpath"
			body = part
		default:
			name = "css"
			body = part
		}

		capture := false
		if strings.HasPrefix(name, "*") {
			capture = true
			name = name[1:]
		}

		return &SelectorPart{Name: name, Body: body}, capture
	}

	var (
		start, index int
		quote        rune
	)

	appendPart := func(start, end int) error {
		p, capture := parsePart(s.Selector[start:end])
		// Skip empty segments between `>>`, e.g., when there are consecutive `>>` or leading/trailing `>>`.
		if p == nil {
			return nil
		}
		return s.appendPart(p, capture)
	}

	if !strings.Contains(s.Selector, ">>") {
		return appendPart(0, len(s.Selector))
	}

	shouldIgnoreTextSelectorQuote := func(start, end int) bool {
		prefix := s.Selector[start:end]
		if match := reTextSelector.FindStringSubmatch(prefix); match != nil && match[1] != "" {
			return true
		}
		return false
	}

	for index < len(s.Selector) {
		c := rune(s.Selector[index])
		switch {
		case c == '\\' && index+1 < len(s.Selector):
			index += 2
		case c == quote:
			quote = 0
			index++
		case quote == 0 && (c == '"' || c == '\'' || c == '`') && !shouldIgnoreTextSelectorQuote(start, index):
			quote = c
			index++
		case quote == 0 && c == '>' && index+1 < len(s.Selector) && s.Selector[index+1] == '>':
			if err := appendPart(start, index); err != nil {
				return err
			}
			index += 2
			start = index
		default:
			index++
		}
	}

	return appendPart(start, index)
}
