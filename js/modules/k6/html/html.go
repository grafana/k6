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
	"errors"

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

		case "option":
			return s.rt.ToValue(optionVal(s.sel))

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

func (s Selection) Closest(selector string) Selection {
	return Selection{s.rt, s.sel.Closest(selector)}
}

func (s Selection) Children(def ...string) Selection {
	if len(def) == 0 {
		return Selection{s.rt, s.sel.Children()}
	} else {
		return Selection{s.rt, s.sel.ChildrenFiltered(def[0])}
	}
}

func (s Selection) Contents() Selection {
	return Selection{s.rt, s.sel.Contents()}
}

func (s Selection) Each(v goja.Value) Selection {
	gojaFn, isFn := goja.AssertFunction(v)
	if isFn {
		// TODO change goquery.Selection arg to html.Node
		fn := func(idx int, sel *goquery.Selection) {
			gojaFn(v, s.rt.ToValue(idx), s.rt.ToValue(sel))
		}
		return Selection{s.rt, s.sel.Each(fn)}
	} else {
		panic(s.rt.NewGoError(errors.New("Argument to each() must be a function")))
		return s
	}
}

func (s Selection) End() Selection {
	return Selection{s.rt, s.sel.End()}
}

func (s Selection) buildMatcher(v goja.Value, gojaFn goja.Callable) func (int, *goquery.Selection) bool {
	return func(idx int, sel *goquery.Selection) bool {
		fnRes, fnErr := gojaFn(v, s.rt.ToValue(idx), s.rt.ToValue(sel))
		return fnErr == nil && fnRes.ToBoolean()
	}
}

func (s Selection) Filter(v goja.Value) Selection {
	gojaFn, isFn := goja.AssertFunction(v)
	if isFn {
		return Selection{s.rt, s.sel.FilterFunction(s.buildMatcher(v, gojaFn))}
	} else {
		return Selection{s.rt, s.sel.Filter(v.String())}
	}
}

func (s Selection) Is(v goja.Value) bool {
	gojaFn, isFn := goja.AssertFunction(v)
	if isFn {
		return s.sel.IsFunction(s.buildMatcher(v, gojaFn))
	} else {
		return s.sel.Is(v.String())
	}
}

func (s Selection) Eq(idx int) Selection {
	return Selection{s.rt, s.sel.Eq(idx)}
}

func (s Selection) First() Selection {
	return Selection{s.rt, s.sel.First()}
}

func (s Selection) Last() Selection {
	return Selection{s.rt, s.sel.Last()}
}

func (s Selection) Has(v goja.Value) Selection {
	// TODO match against Dom/Node items
	return Selection{s.rt, s.sel.Has(v.String())}
}

func (s Selection) Map(v goja.Value) (result [] string) {
	gojaFn, isFn := goja.AssertFunction(v)
	if isFn {
		fn := func(idx int, sel *goquery.Selection) string {
			if fnRes, fnErr := gojaFn(v, s.rt.ToValue(idx), s.rt.ToValue(sel)); fnErr == nil {
				return fnRes.String()
			} else {
				return ""
			}
		}
		return s.sel.Map(fn)
	} else {
		panic(s.rt.NewGoError(errors.New("Argument to map() must be a function")))
		return nil
	}
}

func (s Selection) Not(v goja.Value) Selection {
	gojaFn, isFn := goja.AssertFunction(v)
	if isFn {
		return Selection{s.rt, s.sel.NotFunction(s.buildMatcher(v, gojaFn))}
	}

	val := v.Export()
	switch val.(type) {
		case Selection:
			return Selection{s.rt, s.sel.NotSelection(val.(Selection).sel)}

		case string:
			return Selection{s.rt, s.sel.Not(val.(string))}

		default:
			return Selection{s.rt, s.sel.Not(v.String())}
	}
}

func (s Selection) adjacent(unfiltered func () *goquery.Selection,
							filtered func(string) *goquery.Selection,
							def ...string) Selection {
	if(len(def) == 0) {
		return Selection{s.rt, unfiltered()}
	} else {
		return Selection{s.rt, filtered(def[0])}
	}
}

func (s Selection) Next(def ...string) Selection {
	return s.adjacent(s.sel.Next, s.sel.NextFiltered, def...)
}


func (s Selection) NextAll(def ...string) Selection {
	return s.adjacent(s.sel.NextAll, s.sel.NextAllFiltered, def...)
}


func (s Selection) Prev(def ...string) Selection {
	return s.adjacent(s.sel.Prev, s.sel.PrevFiltered, def...)
}


func (s Selection) PrevAll(def ...string) Selection {
	return s.adjacent(s.sel.PrevAll, s.sel.PrevAllFiltered, def...)
}


func (s Selection) Parent(def ...string) Selection {
	return s.adjacent(s.sel.Parent, s.sel.ParentFiltered, def...)
}

func (s Selection) Parents(def ...string) Selection {
	return s.adjacent(s.sel.Parents, s.sel.ParentsFiltered, def...)
}

func (s Selection) adjacentUntil(until func (string) *goquery.Selection,
								 untilSelection func(*goquery.Selection) *goquery.Selection,
								 filteredUntil func(string, string) *goquery.Selection,
								 filteredUntilSelection func(string, *goquery.Selection) *goquery.Selection,
								 def ...goja.Value) Selection {
	// empty selector to nextuntil and prevuntil matches jquery api and has same effect as prevAll and nextAll
	// relies on goquery.compileMatcher retrning a matcher which fails all matches when invalid selector given to cascadia.compile
	if(len(def) == 0) {
		return Selection{s.rt, until("")}
	}

	selector := def[0].Export()

	if(len(def) == 1) {
		switch selector.(type) {
			case string:
				return Selection{s.rt, until(selector.(string))}

			case Selection:
				return Selection{s.rt, untilSelection(selector.(Selection).sel)}

			default:
				return Selection{s.rt, until("")}
		}
	}

	filter := def[1].String()

	switch selector.(type) {
		case string:
			return Selection{s.rt, filteredUntil(filter, selector.(string))}

		case Selection:
			return Selection{s.rt, filteredUntilSelection(filter, selector.(Selection).sel)}

		default:
			return Selection{s.rt, filteredUntil(filter, "")}
	}
}

// prevUntil, nextUntil and parentsUntil support two args based on jquery api
// 1st arg is either a selector string or goquery.selection.
// 2nd arg is filter selector
// if 1st arg is nil or blank string then behaviour is similar to prevAll or nextAll
func (s Selection) PrevUntil(def ...goja.Value) Selection {
	return s.adjacentUntil(
		s.sel.PrevUntil,
		s.sel.PrevUntilSelection,
		s.sel.PrevFilteredUntil,
		s.sel.PrevFilteredUntilSelection,
		def...
	)
}

func (s Selection) NextUntil(def ...goja.Value) Selection {
	return s.adjacentUntil(
		s.sel.NextUntil,
		s.sel.NextUntilSelection,
		s.sel.NextFilteredUntil,
		s.sel.NextFilteredUntilSelection,
		def...
	)
}

func (s Selection) ParentsUntil(def ...goja.Value) Selection {
	return s.adjacentUntil(
		s.sel.ParentsUntil,
		s.sel.ParentsUntilSelection,
		s.sel.ParentsFilteredUntil,
		s.sel.ParentsFilteredUntilSelection,
		def...
	)
}

func (s Selection) Size() int {
	return s.sel.Length()
}
