package js

import (
	// "github.com/robertkrimen/otto"
	"net/http"
	"strings"
)

type HTTPResponse struct {
	Status int
}

func (a JSAPI) HTTPRequest(method, urlStr, body string, params map[string]interface{}) map[string]interface{} {
	req, err := http.NewRequest(method, urlStr, strings.NewReader(body))
	if err != nil {
		throw(a.vu.vm, err)
	}

	res, err := a.vu.HTTPClient.Do(req)
	if err != nil {
		throw(a.vu.vm, err)
	}

	return map[string]interface{}{
		"status": res.StatusCode,
	}
}
