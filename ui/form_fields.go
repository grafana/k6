/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package ui

import (
	"strings"

	"github.com/pkg/errors"
)

var _ Field = StringField{}

type StringField struct {
	Key     string
	Label   string
	Default string

	// Length constraints.
	Min, Max int
}

func (f StringField) GetKey() string {
	return f.Key
}

func (f StringField) GetLabel() string {
	return f.Label
}

func (f StringField) GetLabelExtra() string {
	return f.Default
}

func (f StringField) Clean(s string) (interface{}, error) {
	s = strings.TrimSpace(s)
	if f.Min != 0 && len(s) < f.Min {
		return nil, errors.Errorf("invalid input, min length is %d", f.Min)
	}
	if f.Max != 0 && len(s) > f.Max {
		return nil, errors.Errorf("invalid input, max length is %d", f.Max)
	}
	if s == "" {
		s = f.Default
	}
	return s, nil
}
