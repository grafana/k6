package js

import (
	"github.com/robertkrimen/otto"
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
