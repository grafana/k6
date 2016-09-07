package api

import (
	"encoding/json"
	"errors"
	"github.com/loadimpact/speedboat/lib"
	"github.com/manyminds/api2go"
	"github.com/manyminds/api2go/jsonapi"
	"io/ioutil"
	"net/http"
	"net/url"
)

var (
	errNoAddress = errors.New("no address given")
)

type Client struct {
	BaseURL url.URL
	Client  http.Client
}

func NewClient(addr string) (*Client, error) {
	if addr == "" {
		return nil, errNoAddress
	}

	return &Client{
		BaseURL: url.URL{Scheme: "http", Host: addr},
		Client:  http.Client{},
	}, nil
}

func (c *Client) request(method, path string) ([]byte, error) {
	relative := url.URL{Path: path}
	req := http.Request{
		Method: method,
		URL:    c.BaseURL.ResolveReference(&relative),
	}
	res, err := c.Client.Do(&req)
	if err != nil {
		return nil, err
	}

	body, _ := ioutil.ReadAll(res.Body)
	res.Body.Close()

	if res.StatusCode >= 400 {
		var envelope api2go.HTTPError
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, err
		}
		if len(envelope.Errors) == 0 {
			return nil, errors.New("Unknown error")
		}
		return nil, errors.New(envelope.Errors[0].Title)
	}

	return body, nil
}

func (c *Client) call(method, path string, out interface{}) error {
	body, err := c.request(method, path)
	if err != nil {
		return err
	}

	return jsonapi.Unmarshal(body, out)
}

func (c *Client) Ping() error {
	_, err := c.request("GET", "/ping")
	if err != nil {
		return err
	}
	return nil
}

// Status returns the status of the currently running test.
func (c *Client) Status() (lib.Status, error) {
	var status lib.Status
	if err := c.call("GET", "/v1/status", &status); err != nil {
		return status, err
	}
	return status, nil
}

// Scales the currently running test.
func (c *Client) Scale(vus int64) error {
	// u := url.URL{Path: "/v1/scale", RawQuery: fmt.Sprintf("vus=%d", vus)}
	// if err := c.call("POST", u, nil); err != nil {
	// 	return err
	// }
	return nil
}

// Aborts the currently running test.
func (c *Client) Abort() error {
	// if err := c.call("POST", url.URL{Path: "/v1/abort"}, nil); err != nil {
	// 	return err
	// }
	return nil
}
