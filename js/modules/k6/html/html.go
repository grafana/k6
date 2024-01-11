package html

import (
	"errors"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dop251/goja"
	gohtml "golang.org/x/net/html"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

// RootModule is the global module object type. It is instantiated once per test
// run and will be used to create k6/html module instances for each VU.
type RootModule struct{}

// ModuleInstance represents an instance of the HTML module for every VU.
type ModuleInstance struct {
	vu         modules.VU
	rootModule *RootModule
	exports    *goja.Object
}

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

// New returns a pointer to a new HTML RootModule.
func New() *RootModule {
	return &RootModule{}
}

// Exports returns the JS values this module exports.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Default: mi.exports,
	}
}

// NewModuleInstance returns an HTML module instance for each VU.
func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	rt := vu.Runtime()
	mi := &ModuleInstance{
		vu:         vu,
		rootModule: r,
		exports:    rt.NewObject(),
	}
	if err := mi.exports.Set("parseHTML", mi.parseHTML); err != nil {
		common.Throw(rt, err)
	}
	return mi
}

func (mi *ModuleInstance) parseHTML(src string) (Selection, error) {
	return ParseHTML(mi.vu.Runtime(), src)
}

// ParseHTML parses the provided HTML source into a Selection object.
func ParseHTML(rt *goja.Runtime, src string) (Selection, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(src))
	if err != nil {
		return Selection{}, err
	}
	return Selection{rt: rt, sel: doc.Selection}, nil
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
	nodeFilter func(...*gohtml.Node) *goquery.Selection,
) Selection {
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
		err := fmt.Errorf("invalid argument, cannot use a '%T' as a selector", arg)
		panic(s.rt.NewGoError(err))
	}
}

func (s Selection) adjacent(unfiltered func() *goquery.Selection,
	filtered func(string) *goquery.Selection,
	def ...string,
) Selection {
	if len(def) > 0 {
		return Selection{s.rt, filtered(def[0]), s.URL}
	}

	return Selection{s.rt, unfiltered(), s.URL}
}

func (s Selection) adjacentUntil(until func(string) *goquery.Selection,
	untilSelection func(*goquery.Selection) *goquery.Selection,
	filteredUntil func(string, string) *goquery.Selection,
	filteredUntilSelection func(string, *goquery.Selection) *goquery.Selection,
	def ...goja.Value,
) Selection {
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

	err := fmt.Errorf("invalid argument, cannot use a '%T' as a selector", def[0].Export())
	panic(s.rt.NewGoError(err))
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

// PrevUntil returns all preceding siblings of each element up to but not including the element matched by the selector.
// The arguments are:
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

// NextUntil returns all following siblings of each element up to but not including the element matched by the selector.
// The arguments are:
// 1st argument is the selector. Either a selector string, a Selection object, or nil
// 2nd argument is the filter. Either a selector string or nil/undefined
func (s Selection) NextUntil(def ...goja.Value) Selection {
	return s.adjacentUntil(
		s.sel.NextUntil,
		s.sel.NextUntilSelection,
		s.sel.NextFilteredUntil,
		s.sel.NextFilteredUntilSelection,
		def...,
	)
}

// ParentsUntil returns the ancestors of each element in the current set of matched elements,
// up to but not including the element matched by the selector
// The arguments are:
// 1st argument is the selector. Either a selector string, a Selection object, or nil
// 2nd argument is the filter. Either a selector string or nil/undefined
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
		common.Throw(s.rt, errors.New("the argument to each() must be a function"))
	}

	fn := func(idx int, sel *goquery.Selection) {
		if _, err := gojaFn(v, s.rt.ToValue(idx), selToElement(Selection{s.rt, s.sel.Eq(idx), s.URL})); err != nil {
			common.Throw(s.rt, fmt.Errorf("the function passed to each() failed: %w", err))
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
		common.Throw(s.rt, errors.New("the argument to filter() must be a function, a selector or a selection"))
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
			common.Throw(s.rt, errors.New("the argument to is() must be a function, a selector or a selection"))
		}

		return s.sel.IsFunction(s.buildMatcher(v, gojaFn))
	}
}

// Map implements ES5 Array.prototype.map
func (s Selection) Map(v goja.Value) []goja.Value {
	gojaFn, isFn := goja.AssertFunction(v)
	if !isFn {
		common.Throw(s.rt, errors.New("the argument to map() must be a function"))
	}

	var values []goja.Value
	s.sel.Each(func(idx int, sel *goquery.Selection) {
		selection := &Selection{sel: sel, URL: s.URL, rt: s.rt}

		if fnRes, fnErr := gojaFn(v, s.rt.ToValue(idx), s.rt.ToValue(selection)); fnErr == nil {
			values = append(values, fnRes)
		}
	})

	return values
}

func (s Selection) Slice(start int, def ...int) Selection {
	// We are forced to check that def[0] is inferior to the length of the array
	// otherwise the s.sel.Slice panics. Besides returning the whole array when
	// the end value for slicing is superior to the array length is standard in js.
	if len(def) > 0 && def[0] < len(s.sel.Nodes) {
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
		}
		return goja.Undefined()
	}
	data := make(map[string]interface{})
	for _, attr := range s.sel.Nodes[0].Attr {
		if strings.HasPrefix(attr.Key, "data-") && len(attr.Key) > 5 {
			data[attrToProperty(attr.Key[5:])] = convertDataAttrVal(attr.Val)
		}
	}
	return s.rt.ToValue(data)
}
