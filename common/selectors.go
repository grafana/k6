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

/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
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

func (s *Selector) parse() error {
	parsePart := func(selector string, start, index int) (*SelectorPart, bool) {
		part := strings.TrimSpace(selector[start:index])
		eqIndex := strings.Index(part, "=")
		var name, body string

		if eqIndex != -1 && reQueryEngine.Match([]byte(strings.TrimSpace(part[0:eqIndex]))) {
			name = strings.TrimSpace(part[0:eqIndex])
			body = part[eqIndex+1:]
		} else if len(part) > 1 && part[0] == '"' && part[len(part)-1] == '"' {
			name = "text"
			body = part
		} else if len(part) > 1 && part[0] == '\'' && part[len(part)-1] == '\'' {
			name = "text"
			body = part
		} else if reXPathSelector.Match([]byte(part)) || strings.HasPrefix(part, "..") {
			// If selector starts with '//' or '//' prefixed with multiple opening
			// parenthesis, consider xpath. @see https://github.com/microsoft/playwright/issues/817
			// If selector starts with '..', consider xpath as well.
			name = "xpath"
			body = part
		} else {
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
		if c == '\\' && index+1 < len(s.Selector) {
			index += 2
		} else if c == quote {
			quote = byte(0)
			index++
		} else if quote == 0 && (c == '"' || c == '\'' || c == '`') {
			quote = c
			index++
		} else if quote == 0 && c == '>' && s.Selector[index+1] == '>' {
			part, capture := parsePart(s.Selector, start, index)
			err := s.appendPart(part, capture)
			if err != nil {
				return err
			}
			index += 2
			start = index
		} else {
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
