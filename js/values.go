package js

import (
	"encoding/json"
	"gopkg.in/olebedev/go-duktape.v2"
)

func argNumber(js *duktape.Context, index int) float64 {
	if js.GetTopIndex() < index {
		return 0
	}

	return js.ToNumber(index)
}

func argString(js *duktape.Context, index int) string {
	if js.GetTopIndex() < index {
		return ""
	}

	return js.ToString(index)
}

func argJSON(js *duktape.Context, index int, out interface{}) error {
	if js.GetTopIndex() < index {
		return nil
	}

	js.JsonEncode(index)
	str := js.GetString(index)
	return json.Unmarshal([]byte(str), out)
}

func pushObject(js *duktape.Context, obj interface{}, t string) error {
	s, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	js.PushString(string(s))
	js.JsonDecode(-1)

	if t != "" {
		js.PushGlobalObject()
		{
			js.GetPropString(-1, t)
			js.SetPrototype(-3)
		}
		js.Pop()
	}

	return nil
}
