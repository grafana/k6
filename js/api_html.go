package js

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/robertkrimen/otto"
	"strings"
)

func (a JSAPI) HTMLParse(src string) *goquery.Selection {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(src))
	if err != nil {
		throw(a.vu.vm, err)
	}
	return doc.Selection
}

func (a JSAPI) HTMLSelectionAddSelection(vA, vB otto.Value) *goquery.Selection {
	iA, err := vA.Export()
	if err != nil {
		throw(a.vu.vm, err)
	}
	selA, ok := iA.(*goquery.Selection)
	if !ok {
		panic(a.vu.vm.MakeTypeError("HTMLSelectionAddSelection argument A is not a *goquery.Selection"))
	}

	iB, err := vB.Export()
	if err != nil {
		throw(a.vu.vm, err)
	}
	selB, ok := iB.(*goquery.Selection)
	if !ok {
		panic(a.vu.vm.MakeTypeError("HTMLSelectionAddSelection argument B is not a *goquery.Selection"))
	}

	return selA.AddSelection(selB)
}
