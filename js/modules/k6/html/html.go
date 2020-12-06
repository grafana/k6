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
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dop251/goja"
	"github.com/pkg/errors"
	gohtml "golang.org/x/net/html"

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/internal/modules"
)

func init() {
	modules.Register("k6/html", New())
}

type HTML struct{}

func New() *HTML {
	return &HTML{}
}

func (HTML) ParseHTML(ctx context.Context, src string) (Selection, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(src))
	if err != nil {
		return Selection{}, err
	}
	return Selection{rt: common.GetRuntime(ctx), sel: doc.Selection}, nil
}

type Selection struct {
	rt  *goja.Runtime
	sel *goquery.Selection
	URL string `json:"url"`
}

func (s Selection) emptySelection() Selection {
	// Goquery has no direct way to return an empty selection apart from asking for an out of bounds item.
	return s.Eq(s.Size())
}

func (s Selection) buildMatcher(v goja.Value, gojaFn goja.Callable) func(int, *goquery.Selection) bool {
	return func(idx int, sel *goquery.Selection) bool {
		fnRes, fnErr := gojaFn(v, s.rt.ToValue(idx), s.rt.ToValue(sel))
		if fnErr != nil {
			common.Throw(s.rt, fnErr)
		}

		return fnRes.ToBoolean()
	}
}

func (s Selection) varargFnCall(arg interface{},
	strFilter func(string) *goquery.Selection,
	selFilter func(*goquery.Selection) *goquery.Selection,
	nodeFilter func(...*gohtml.Node) *goquery.Selection) Selection {

	switch v := arg.(type) {
	case Selection:
		return Selection{s.rt, selFilter(v.sel), s.URL}

	case string:
		return Selection{s.rt, strFilter(v), s.URL}

	case Element:
		return Selection{s.rt, nodeFilter(v.node), s.URL}

	case goja.Value:
		return s.varargFnCall(v.Export(), strFilter, selFilter, nodeFilter)

	default:
		errmsg := fmt.Sprintf("Invalid argument: Cannot use a %T as a selector", arg)
		panic(s.rt.NewGoError(errors.New(errmsg)))
	}
}

func (s Selection) adjacent(unfiltered func() *goquery.Selection,
	filtered func(string) *goquery.Selection,
	def ...string) Selection {
	if len(def) > 0 {
		return Selection{s.rt, filtered(def[0]), s.URL}
	}

	return Selection{s.rt, unfiltered(), s.URL}
}

func (s Selection) adjacentUntil(until func(string) *goquery.Selection,
	untilSelection func(*goquery.Selection) *goquery.Selection,
	filteredUntil func(string, string) *goquery.Selection,
	filteredUntilSelection func(string, *goquery.Selection) *goquery.Selection,
	def ...goja.Value) Selection {

	switch len(def) {
	case 0:
		return Selection{s.rt, until(""), s.URL}
	case 1:
		switch selector := def[0].Export().(type) {
		case string:
			return Selection{s.rt, until(selector), s.URL}

		case Selection:
			return Selection{s.rt, untilSelection(selector.sel), s.URL}

		case nil:
			return Selection{s.rt, until(""), s.URL}
		}
	case 2:
		filter := def[1].String()

		switch selector := def[0].Export().(type) {
		case string:
			return Selection{s.rt, filteredUntil(filter, selector), s.URL}

		case Selection:
			return Selection{s.rt, filteredUntilSelection(filter, selector.sel), s.URL}

		case nil:
			return Selection{s.rt, filteredUntil(filter, ""), s.URL}
		}
	}

	errmsg := fmt.Sprintf("Invalid argument: Cannot use a %T as a selector", def[0].Export())
	panic(s.rt.NewGoError(errors.New(errmsg)))
}

func (s Selection) Add(arg interface{}) Selection {
	return s.varargFnCall(arg, s.sel.Add, s.sel.AddSelection, s.sel.AddNodes)
}

func (s Selection) Find(arg interface{}) Selection {
	return s.varargFnCall(arg, s.sel.Find, s.sel.FindSelection, s.sel.FindNodes)
}

func (s Selection) Closest(arg interface{}) Selection {
	return s.varargFnCall(arg, s.sel.Closest, s.sel.ClosestSelection, s.sel.ClosestNodes)
}

func (s Selection) Has(arg interface{}) Selection {
	return s.varargFnCall(arg, s.sel.Has, s.sel.HasSelection, s.sel.HasNodes)
}

func (s Selection) Not(v goja.Value) Selection {
	gojaFn, isFn := goja.AssertFunction(v)
	if !isFn {
		return s.varargFnCall(v, s.sel.Not, s.sel.NotSelection, s.sel.NotNodes)
	}

	return Selection{s.rt, s.sel.NotFunction(s.buildMatcher(v, gojaFn)), s.URL}
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

func (s Selection) Siblings(def ...string) Selection {
	return s.adjacent(s.sel.Siblings, s.sel.SiblingsFiltered, def...)
}

// prevUntil, nextUntil and parentsUntil support two arguments with mutable type.
// 1st argument is the selector. Either a selector string, a Selection object, or nil
// 2nd argument is the filter. Either a selector string or nil/undefined
func (s Selection) PrevUntil(def ...goja.Value) Selection {
	return s.adjacentUntil(
		s.sel.PrevUntil,
		s.sel.PrevUntilSelection,
		s.sel.PrevFilteredUntil,
		s.sel.PrevFilteredUntilSelection,
		def...,
	)
}

func (s Selection) NextUntil(def ...goja.Value) Selection {
	return s.adjacentUntil(
		s.sel.NextUntil,
		s.sel.NextUntilSelection,
		s.sel.NextFilteredUntil,
		s.sel.NextFilteredUntilSelection,
		def...,
	)
}

func (s Selection) ParentsUntil(def ...goja.Value) Selection {
	return s.adjacentUntil(
		s.sel.ParentsUntil,
		s.sel.ParentsUntilSelection,
		s.sel.ParentsFilteredUntil,
		s.sel.ParentsFilteredUntilSelection,
		def...,
	)
}

func (s Selection) Size() int {
	return s.sel.Length()
}

func (s Selection) End() Selection {
	return Selection{s.rt, s.sel.End(), s.URL}
}

func (s Selection) Eq(idx int) Selection {
	return Selection{s.rt, s.sel.Eq(idx), s.URL}
}

func (s Selection) First() Selection {
	return Selection{s.rt, s.sel.First(), s.URL}
}

func (s Selection) Last() Selection {
	return Selection{s.rt, s.sel.Last(), s.URL}
}

func (s Selection) Contents() Selection {
	return Selection{s.rt, s.sel.Contents(), s.URL}
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

// nolint: goconst
func (s Selection) Val() goja.Value {
	switch goquery.NodeName(s.sel) {
	case InputTagName:
		val, exists := s.sel.Attr("value")
		if !exists {
			inputType, _ := s.sel.Attr("type")
			if inputType == "radio" || inputType == "checkbox" {
				val = "on"
			} else {
				val = ""
			}
		}
		return s.rt.ToValue(val)

	case ButtonTagName:
		val, exists := s.sel.Attr("value")
		if !exists {
			val = ""
		}
		return s.rt.ToValue(val)

	case TextAreaTagName:
		return s.Html()

	case OptionTagName:
		return s.rt.ToValue(valueOrHTML(s.sel))

	case SelectTagName:
		selected := s.sel.First().Find("option[selected]")
		if _, exists := s.sel.Attr("multiple"); exists {
			return s.rt.ToValue(selected.Map(func(idx int, opt *goquery.Selection) string { return valueOrHTML(opt) }))
		}

		return s.rt.ToValue(valueOrHTML(selected))

	default:
		return goja.Undefined()
	}
}

func (s Selection) Children(def ...string) Selection {
	if len(def) == 0 {
		return Selection{s.rt, s.sel.Children(), s.URL}
	}

	return Selection{s.rt, s.sel.ChildrenFiltered(def[0]), s.URL}
}

func (s Selection) Each(v goja.Value) Selection {
	gojaFn, isFn := goja.AssertFunction(v)
	if !isFn {
		common.Throw(s.rt, errors.New("Argument to each() must be a function."))
	}

	fn := func(idx int, sel *goquery.Selection) {
		if _, err := gojaFn(v, s.rt.ToValue(idx), selToElement(Selection{s.rt, s.sel.Eq(idx), s.URL})); err != nil {
			common.Throw(s.rt, errors.Wrap(err, "Function passed to each() failed."))
		}
	}

	return Selection{s.rt, s.sel.Each(fn), s.URL}
}

func (s Selection) Filter(v goja.Value) Selection {
	switch val := v.Export().(type) {
	case string:
		return Selection{s.rt, s.sel.Filter(val), s.URL}

	case Selection:
		return Selection{s.rt, s.sel.FilterSelection(val.sel), s.URL}
	}

	gojaFn, isFn := goja.AssertFunction(v)
	if !isFn {
		common.Throw(s.rt, errors.New("Argument to filter() must be a function, a selector or a selection"))
	}

	return Selection{s.rt, s.sel.FilterFunction(s.buildMatcher(v, gojaFn)), s.URL}
}

func (s Selection) Is(v goja.Value) bool {
	switch val := v.Export().(type) {
	case string:
		return s.sel.Is(val)

	case Selection:
		return s.sel.IsSelection(val.sel)

	default:
		gojaFn, isFn := goja.AssertFunction(v)
		if !isFn {
			common.Throw(s.rt, errors.New("Argument to is() must be a function, a selector or a selection"))
		}

		return s.sel.IsFunction(s.buildMatcher(v, gojaFn))
	}
}

// Map implements ES5 Array.prototype.map
func (s Selection) Map(v goja.Value) []string {
	gojaFn, isFn := goja.AssertFunction(v)
	if !isFn {
		common.Throw(s.rt, errors.New("Argument to map() must be a function"))
	}

	fn := func(idx int, sel *goquery.Selection) string {
		selection := &Selection{sel: sel, URL: s.URL, rt: s.rt}
		if fnRes, fnErr := gojaFn(v, s.rt.ToValue(idx), s.rt.ToValue(selection)); fnErr == nil {
			return fnRes.String()
		}
		return ""
	}

	return s.sel.Map(fn)
}

func (s Selection) Slice(start int, def ...int) Selection {
	if len(def) > 0 {
		return Selection{s.rt, s.sel.Slice(start, def[0]), s.URL}
	}

	return Selection{s.rt, s.sel.Slice(start, s.sel.Length()), s.URL}
}

func (s Selection) Get(def ...int) goja.Value {
	switch {
	case len(def) == 0:
		var items []goja.Value
		for i := 0; i < len(s.sel.Nodes); i++ {
			items = append(items, selToElement(s.Eq(i)))
		}
		return s.rt.ToValue(items)

	case def[0] < s.sel.Length() && def[0] > -s.sel.Length():
		return selToElement(s.Eq(def[0]))

	default:
		return goja.Undefined()
	}
}

func (s Selection) ToArray() []Selection {
	items := make([]Selection, len(s.sel.Nodes))
	for i := range s.sel.Nodes {
		items[i] = Selection{s.rt, s.sel.Eq(i), s.URL}
	}
	return items
}

func (s Selection) Index(def ...goja.Value) int {
	if len(def) == 0 {
		return s.sel.Index()
	}

	switch v := def[0].Export().(type) {
	case Selection:
		return s.sel.IndexOfSelection(v.sel)

	case string:
		return s.sel.IndexSelector(v)

	case Element:
		return s.sel.IndexOfNode(v.node)

	default:
		return -1
	}
}

// When 0 arguments: Read all data from attributes beginning with "data-".
// When 1 argument: Append argument to "data-" then find for a matching attribute
func (s Selection) Data(def ...string) goja.Value {
	if s.sel.Length() == 0 || len(s.sel.Nodes[0].Attr) == 0 {
		return goja.Undefined()
	}

	if len(def) > 0 {
		val, exists := s.sel.Attr("data-" + propertyToAttr(def[0]))
		if exists {
			return s.rt.ToValue(convertDataAttrVal(val))
		} else {
			return goja.Undefined()
		}
	} else {
		data := make(map[string]interface{})
		for _, attr := range s.sel.Nodes[0].Attr {
			if strings.HasPrefix(attr.Key, "data-") && len(attr.Key) > 5 {
				data[attrToProperty(attr.Key[5:])] = convertDataAttrVal(attr.Val)
			}
		}
		return s.rt.ToValue(data)
	}
}
