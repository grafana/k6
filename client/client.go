package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/loadimpact/speedboat/lib"
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

func New(addr string) (*Client, error) {
	if addr == "" {
		return nil, errNoAddress
	}

	return &Client{
		BaseURL: url.URL{Scheme: "http", Host: addr},
		Client:  http.Client{},
	}, nil
}

func (c *Client) call(method string, relative url.URL, out interface{}) error {
	req := http.Request{
		Method: method,
		URL:    c.BaseURL.ResolveReference(&relative),
	}
	res, err := c.Client.Do(&req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		var envelope struct {
			Error string `json:"error"`
		}
		body, _ := ioutil.ReadAll(res.Body)
		if err := json.Unmarshal(body, &envelope); err != nil {
			return err
		}
		if envelope.Error == "" {
			envelope.Error = res.Status
		}
		return errors.New(envelope.Error)
	}

	if out == nil {
		return nil
	}

	body, _ := ioutil.ReadAll(res.Body)
	if err := json.Unmarshal(body, out); err != nil {
		return err
	}

	return nil
}

func (c *Client) Ping() error {
	if err := c.call("GET", url.URL{Path: "/ping"}, nil); err != nil {
		return err
	}
	return nil
}

// Status returns the status of the currently running test.
func (c *Client) Status() (lib.Status, error) {
	var status lib.Status
	if err := c.call("GET", url.URL{Path: "/v1/status"}, &status); err != nil {
		return status, err
	}
	return status, nil
}

// Scales the currently running test.
func (c *Client) Scale(vus int64) error {
	u := url.URL{Path: "/v1/scale", RawQuery: fmt.Sprintf("vus=%d", vus)}
	if err := c.call("POST", u, nil); err != nil {
		return err
	}
	return nil
}

// Aborts the currently running test.
func (c *Client) Abort() error {
	if err := c.call("POST", url.URL{Path: "/v1/abort"}, nil); err != nil {
		return err
	}
	return nil
}
