package postman

import (
	"bytes"
	"errors"
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
}

func MakeEndpoint(i Item) (Endpoint, error) {
	if i.Request.URL == "" {
		return Endpoint{}, ErrItemHasNoRequest
	}

	u, err := url.Parse(i.Request.URL)
	if err != nil {
		return Endpoint{}, err
	}

	header := make(http.Header)
	for _, item := range i.Request.Header {
		header[item.Key] = append(header[item.Key], item.Value)
	}

	var body []byte
	switch i.Request.Body.Mode {
	case "raw":
		body = []byte(i.Request.Body.Raw)
	case "urlencoded":
		values := make(url.Values)
		for _, field := range i.Request.Body.URLEncoded {
			if !field.Enabled {
				continue
			}
			values[field.Key] = append(values[field.Key], field.Value)
		}
		body = []byte(values.Encode())
	case "formdata":
		body = make([]byte, 0)
		w := multipart.NewWriter(bytes.NewBuffer(body))
		for _, field := range i.Request.Body.FormData {
			if !field.Enabled {
				continue
			}

			if err := w.WriteField(field.Key, field.Value); err != nil {
				return Endpoint{}, err
			}
		}
	}

	return Endpoint{i.Request.Method, u, header, body}, nil
}

func (ep Endpoint) Request() http.Request {
	return http.Request{
		Method: ep.Method,
		URL:    ep.URL,
		Header: ep.Header,
		Body:   ioutil.NopCloser(bytes.NewBuffer(ep.Body)),
	}
}
