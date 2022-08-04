package html

import (
	neturl "net/url"

	"github.com/PuerkitoBio/goquery"
	"github.com/dop251/goja"
)

type FormValue struct {
	Name  string
	Value goja.Value
}

func (s Selection) SerializeArray() []FormValue {
	submittableSelector := "input,select,textarea,keygen"
	var formElements *goquery.Selection
	if s.sel.Is("form") {
		formElements = s.sel.Find(submittableSelector)
	} else {
		formElements = s.sel.Filter(submittableSelector)
	}

	formElements = formElements.FilterFunction(func(_ int, sel *goquery.Selection) bool {
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
			(checked == "checked" ||
				(inputType != "checkbox" && inputType != "radio")) // Must be checked if it is an checkbox or radio
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
