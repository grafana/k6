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
	neturl "net/url"

	"github.com/PuerkitoBio/goquery"
	"github.com/dop251/goja"
)

type FormValue struct {
	Value goja.Value
	Name  string
}

// nolint: goconst
func (s Selection) SerializeArray() []FormValue {
	submittableSelector := "input,select,textarea,keygen"
	var formElements *goquery.Selection
	if s.sel.Is("form") {
		formElements = s.sel.Find(submittableSelector)
	} else {
		formElements = s.sel.Filter(submittableSelector)
	}

	formElements = formElements.FilterFunction(func(i int, sel *goquery.Selection) bool {
		name := sel.AttrOr("name", "")
		inputType := sel.AttrOr("type", "")
		disabled := sel.AttrOr("disabled", "")
		checked := sel.AttrOr("checked", "")

		return name != "" && // Must have a non-empty name
			disabled != "disabled" && // Must not be disabled
			inputType != "submit" && // Must not be a button
			inputType != "button" &&
			inputType != "reset" &&
			inputType != "image" && // Must not be an image or file
			inputType != "file" &&
			(checked == "checked" || (inputType != "checkbox" && inputType != "radio")) // Must be checked if it is an checkbox or radio
	})

	result := make([]FormValue, len(formElements.Nodes))
	formElements.Each(func(i int, sel *goquery.Selection) {
		element := Selection{s.rt, sel, s.URL}
		name, _ := sel.Attr("name")
		result[i] = FormValue{Name: name, Value: element.Val()}
	})
	return result
}

func (s Selection) SerializeObject() map[string]goja.Value {
	formValues := s.SerializeArray()
	result := make(map[string]goja.Value)
	for i := range formValues {
		formValue := formValues[i]
		result[formValue.Name] = formValue.Value
	}

	return result
}

func (s Selection) Serialize() string {
	formValues := s.SerializeArray()
	urlValues := make(neturl.Values, len(formValues))
	for i := range formValues {
		formValue := formValues[i]
		value := formValue.Value.Export()
		switch v := value.(type) {
		case string:
			urlValues.Set(formValue.Name, v)
		case []string:
			urlValues[formValue.Name] = v
		}
	}
	return urlValues.Encode()
}
