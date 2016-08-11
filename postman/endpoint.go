package postman

import (
	"bytes"
	"errors"
	"github.com/robertkrimen/otto"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
)

var (
	ErrItemHasNoRequest = errors.New("can't make an endpoint out of an item with no request")
)

type Endpoint struct {
	Method string
	URL    *url.URL
	Header http.Header
	Body   []byte

	Tests      []*otto.Script
	PreRequest []*otto.Script

	URLString string
}

func MakeEndpoints(c Collection, vm *otto.Otto) ([]Endpoint, error) {
	eps := make([]Endpoint, 0)
	for _, item := range c.Item {
		if err := makeEndpointsFrom(item, vm, &eps); err != nil {
			return eps, err
		}
	}

	return eps, nil
}

func makeEndpointsFrom(i Item, vm *otto.Otto, eps *[]Endpoint) error {
	if i.Request.URL != "" {
		ep, err := MakeEndpoint(i, vm)
		if err != nil {
			return err
		}
		*eps = append(*eps, ep)
	}

	for _, item := range i.Item {
		if err := makeEndpointsFrom(item, vm, eps); err != nil {
			return err
		}
	}

	return nil
}

func MakeEndpoint(i Item, vm *otto.Otto) (Endpoint, error) {
	if i.Request.URL == "" {
		return Endpoint{}, ErrItemHasNoRequest
	}

	endpoint := Endpoint{
		Method:    i.Request.Method,
		URLString: i.Request.URL,
	}

	u, err := url.Parse(i.Request.URL)
	if err != nil {
		return endpoint, err
	}
	endpoint.URL = u

	endpoint.Header = make(http.Header)
	for _, item := range i.Request.Header {
		endpoint.Header[item.Key] = append(endpoint.Header[item.Key], item.Value)
	}

	switch i.Request.Body.Mode {
	case "raw":
		endpoint.Body = []byte(i.Request.Body.Raw)
	case "urlencoded":
		values := make(url.Values)
		for _, field := range i.Request.Body.URLEncoded {
			if !field.Enabled {
				continue
			}
			values[field.Key] = append(values[field.Key], field.Value)
		}
		endpoint.Body = []byte(values.Encode())
	case "formdata":
		endpoint.Body = make([]byte, 0)
		w := multipart.NewWriter(bytes.NewBuffer(endpoint.Body))
		for _, field := range i.Request.Body.FormData {
			if !field.Enabled {
				continue
			}

			if err := w.WriteField(field.Key, field.Value); err != nil {
				return endpoint, err
			}
		}
	}

	if vm != nil {
		for _, event := range i.Event {
			if event.Disabled {
				continue
			}

			script, err := vm.Compile("event", string(event.Script.Exec))
			if err != nil {
				return endpoint, err
			}

			switch event.Listen {
			case "test":
				endpoint.Tests = append(endpoint.Tests, script)
			case "prerequest":
				endpoint.PreRequest = append(endpoint.PreRequest, script)
			}
		}
	}

	return endpoint, nil
}

func (ep Endpoint) Request() http.Request {
	return http.Request{
		Method:        ep.Method,
		URL:           ep.URL,
		Header:        ep.Header,
		Body:          ioutil.NopCloser(bytes.NewBuffer(ep.Body)),
		ContentLength: int64(len(ep.Body)),
	}
}
