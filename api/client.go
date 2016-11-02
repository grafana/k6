package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/stats"
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

func (c *Client) request(method, path string, body []byte) ([]byte, error) {
	relative := url.URL{Path: path}
	req := http.Request{
		Method: method,
		URL:    c.BaseURL.ResolveReference(&relative),
	}
	if body != nil {
		req.ContentLength = int64(len(body))
		req.Body = ioutil.NopCloser(bytes.NewReader(body))
	}

	res, err := c.Client.Do(&req)
	if err != nil {
		return nil, err
	}

	data, _ := ioutil.ReadAll(res.Body)
	res.Body.Close()

	if res.StatusCode >= 400 {
		var envelope api2go.HTTPError
		if err := json.Unmarshal(data, &envelope); err != nil {
			return nil, err
		}
		if len(envelope.Errors) == 0 {
			return nil, errors.New("Unknown error")
		}
		return nil, errors.New(envelope.Errors[0].Title)
	}

	return data, nil
}

func (c *Client) call(method, path string, body []byte, out interface{}) error {
	body, err := c.request(method, path, body)
	if err != nil {
		return err
	}

	return jsonapi.Unmarshal(body, out)
}

func (c *Client) Ping() error {
	_, err := c.request("GET", "/ping", nil)
	if err != nil {
		return err
	}
	return nil
}

// Status returns the status of the currently running test.
func (c *Client) Status() (lib.Status, error) {
	var status lib.Status
	if err := c.call("GET", "/v1/status", nil, &status); err != nil {
		return status, err
	}
	return status, nil
}

// Updates the status of the currently running test.
func (c *Client) UpdateStatus(status lib.Status) (lib.Status, error) {
	data, err := jsonapi.Marshal(status)
	if err != nil {
		return status, err
	}
	if err := c.call("PATCH", "/v1/status", data, &status); err != nil {
		return status, err
	}
	return status, nil
}

// Returns a snapshot of metrics for the currently running test.
func (c *Client) Metrics() ([]stats.Metric, error) {
	var metrics []stats.Metric
	if err := c.call("GET", "/v1/metrics", nil, &metrics); err != nil {
		return metrics, err
	}
	return metrics, nil
}
