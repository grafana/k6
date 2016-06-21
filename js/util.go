package js

import (
	"github.com/robertkrimen/otto"
	"net/url"
	"strings"
)

func paramsFromObject(o *otto.Object) (params HTTPParams, err error) {
	if o == nil {
		return params, nil
	}

	for _, key := range o.Keys() {
		switch key {
		case "quiet":
			v, err := o.Get(key)
			if err != nil {
				return params, err
			}
			quiet, err := v.ToBoolean()
			if err != nil {
				return params, err
			}
			params.Quiet = quiet
		case "headers":
			v, err := o.Get(key)
			if err != nil {
				return params, err
			}
			obj := v.Object()
			if obj == nil {
				continue
			}

			params.Headers = make(map[string]string)
			for _, name := range obj.Keys() {
				hv, err := obj.Get(name)
				if err != nil {
					return params, err
				}
				value, err := hv.ToString()
				if err != nil {
					return params, err
				}
				params.Headers[name] = value
			}
		}
	}

	return params, nil
}

func bodyFromValue(o otto.Value) (string, error) {
	if o.IsUndefined() || o.IsNull() {
		return "", nil
	}

	if o.IsObject() {
		query := make(url.Values)
		obj := o.Object()
		for _, key := range obj.Keys() {
			valObj, _ := obj.Get(key)
			val, err := valObj.ToString()
			if err != nil {
				return "", err
			}
			query.Add(key, val)
		}
		return query.Encode(), nil
	}

	body, err := o.ToString()
	if err != nil {
		return "", err
	}

	return body, nil
}

func putBodyInURL(url, body string) string {
	if body == "" {
		return url
	}

	if !strings.ContainsRune(url, '?') {
		return url + "?" + body
	} else {
		return url + "&" + body
	}
}

func Make(vm *otto.Otto, t string) (*otto.Object, error) {
	val, err := vm.Call("new "+t, nil)
	if err != nil {
		return nil, err
	}

	return val.Object(), nil
}

func jsCustomError(vm *otto.Otto, t string, err error) otto.Value {
	return vm.MakeCustomError(t, err.Error())
}

func jsError(vm *otto.Otto, err error) otto.Value {
	return jsCustomError(vm, "Error", err)
}
