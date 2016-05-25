package js

import (
	"encoding/json"
	"gopkg.in/olebedev/go-duktape.v2"
)

func argNumber(c *duktape.Context, index int) float64 {
	if c.GetTopIndex() < index {
		return 0
	}

	return c.ToNumber(index)
}

func argString(c *duktape.Context, index int) string {
	if c.GetTopIndex() < index {
		return ""
	}

	return c.ToString(index)
}

func argJSON(c *duktape.Context, index int, out interface{}) error {
	if c.GetTopIndex() < index {
		return nil
	}

	c.JsonEncode(index)
	str := c.GetString(index)
	return json.Unmarshal([]byte(str), out)
}
