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

package js

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/robertkrimen/otto"
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
