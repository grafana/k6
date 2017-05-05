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

package html

import (
	"context"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
)

type HTML struct{}

func New() *HTML {
	return &HTML{}
}

func (HTML) ParseHTML(ctx context.Context, src string) (Selection, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(src))
	if err != nil {
		return Selection{}, err
	}
	return Selection{common.GetRuntime(ctx), doc.Selection}, nil
}

type Selection struct {
	rt  *goja.Runtime
	sel *goquery.Selection
}

func (s Selection) Add(arg goja.Value) Selection {
	switch val := arg.Export().(type) {
	case Selection:
		return Selection{s.rt, s.sel.AddSelection(val.sel)}
	default:
		return Selection{s.rt, s.sel.Add(arg.String())}
	}
}

func (s Selection) Find(sel string) Selection {
	return Selection{s.rt, s.sel.Find(sel)}
}

func (s Selection) Text() string {
	return s.sel.Text()
}

func (s Selection) Attr(name string, def ...goja.Value) goja.Value {
	val, exists := s.sel.Attr(name)
	if !exists {
		if len(def) > 0 {
			return def[0]
		}
		return goja.Undefined()
	}
	return s.rt.ToValue(val)
}

func (s Selection) Html() goja.Value {
	val, err := s.sel.Html()
	if err != nil {
		return goja.Undefined()
	}
	return s.rt.ToValue(val)
}

func optionVal(s *goquery.Selection) string {
	val, exists := s.Attr("value")
	if exists {
		return val
	}

	val, err := s.Html()
	if err != nil {
		return ""
	}

	return val
}

func(s Selection) Val() goja.Value {
	switch goquery.NodeName(s.sel) {
		case "input":
			return s.Attr("value")

		case "textarea":
			return s.Html()

		case "button":
			return s.Attr("value")

		case "select":
			selected := s.sel.First().Find("option[selected]")

			if _, exists := s.sel.Attr("multiple"); exists {
				return s.rt.ToValue(selected.Map(func(idx int, opt *goquery.Selection) string { return optionVal(opt) }))
			} else {
				return s.rt.ToValue(optionVal(selected))
			}

		default:
			return goja.Undefined()
	}
}

func (s Selection) Closest(name string) Selection {
	return Selection{s.rt, s.sel.Closest(name)}
}
