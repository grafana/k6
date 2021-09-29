/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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
	"encoding/json"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dop251/goja"
	"github.com/serenize/snaker"
	gohtml "golang.org/x/net/html"
)

func attrToProperty(s string) string {
	if idx := strings.Index(s, "-"); idx != -1 {
		return s[0:idx] + snaker.SnakeToCamel(strings.Replace(s[idx+1:], "-", "_", -1))
	}
	return s
}

func propertyToAttr(attrName string) string {
	return strings.Replace(snaker.CamelToSnake(attrName), "_", "-", -1)
}

func namespaceURI(prefix string) string {
	switch prefix {
	case "svg":
		return "http://www.w3.org/2000/svg"
	case "math":
		return "http://www.w3.org/1998/Math/MathML"
	default:
		return "http://www.w3.org/1999/xhtml"
	}
}

func valueOrHTML(s *goquery.Selection) string {
	if val, exists := s.Attr("value"); exists {
		return val
	}

	if val, err := s.Html(); err == nil {
		return val
	}

	return ""
}

func getHtmlAttr(node *gohtml.Node, name string) *gohtml.Attribute {
	for i := 0; i < len(node.Attr); i++ {
		if node.Attr[i].Key == name {
			return &node.Attr[i]
		}
	}

	return nil
}

func elemList(s Selection) (items []goja.Value) {
	items = make([]goja.Value, s.Size())
	for i := 0; i < s.Size(); i++ {
		items[i] = selToElement(s.Eq(i))
	}
	return items
}

func nodeToElement(e Element, node *gohtml.Node) goja.Value {
	// Goquery does not expose a way to build a goquery.Selection with an arbitrary html.Node.
	// Workaround by adding a node to an empty Selection
	emptySel := e.sel.emptySelection()
	emptySel.sel.Nodes = append(emptySel.sel.Nodes, node)

	return selToElement(emptySel)
}

// Try to read numeric values in data- attributes.
// Return numeric value when the representation is unchanged by conversion to float and back.
// Other potentially numeric values (ie "101.00" "1E02") remain as strings.
func toNumeric(val string) (float64, bool) {
	if fltVal, err := strconv.ParseFloat(val, 64); err != nil {
		return 0, false
	} else if repr := strconv.FormatFloat(fltVal, 'f', -1, 64); repr == val {
		return fltVal, true
	} else {
		return 0, false
	}
}

func convertDataAttrVal(val string) interface{} {
	if len(val) == 0 {
		return goja.Undefined()
	} else if val[0] == '{' || val[0] == '[' {
		var subdata interface{}

		err := json.Unmarshal([]byte(val), &subdata)
		if err == nil {
			return subdata
		} else {
			return val
		}
	} else {
		switch val {
		case "true":
			return true

		case "false":
			return false

		case "null":
			return goja.Undefined()

		case "undefined":
			return goja.Undefined()

		default:
			if fltVal, isOk := toNumeric(val); isOk {
				return fltVal
			} else {
				return val
			}
		}
	}
}
