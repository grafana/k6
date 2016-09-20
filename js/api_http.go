package js

import (
	// "github.com/robertkrimen/otto"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

type HTTPResponse struct {
	Status int
}

func (a JSAPI) HTTPRequest(method, url, body string, params map[string]interface{}) map[string]interface{} {
	bodyReader := io.Reader(nil)
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		throw(a.vu.vm, err)
	}

	res, err := a.vu.HTTPClient.Do(req)
	if err != nil {
		throw(a.vu.vm, err)
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		throw(a.vu.vm, err)
	}
	res.Body.Close()

	return map[string]interface{}{
		"status": res.StatusCode,
		"body":   string(resBody),
	}
}
