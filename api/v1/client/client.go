/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/manyminds/api2go/jsonapi"
	"github.com/sirupsen/logrus"

	v1 "go.k6.io/k6/api/v1"
)

// Client is a simple HTTP client for the REST API.
type Client struct {
	BaseURL    *url.URL
	httpClient *http.Client
	logger     *logrus.Entry
}

// Option function are helpers that enable the flexible configuration of the
// REST API client.
type Option func(*Client)

// New returns a newly configured REST API Client.
func New(base string, options ...Option) (*Client, error) {
	baseURL, err := url.Parse("http://" + base)
	if err != nil {
		return nil, err
	}
	c := &Client{
		BaseURL:    baseURL,
		httpClient: http.DefaultClient,
	}

	for _, option := range options {
		option(c)
	}

	return c, nil
}

// WithHTTPClient configures the supplied HTTP client to be used when making
// REST API requests.
func WithHTTPClient(httpClient *http.Client) Option {
	return Option(func(c *Client) {
		c.httpClient = httpClient
	})
}

// WithLogger sets the specified logger to the client.
func WithLogger(logger *logrus.Entry) Option {
	return Option(func(c *Client) {
		c.logger = logger
	})
}

// CallAPI executes the desired REST API request.
// it's expected that the body and out are the structs that follows the JSON:API
func (c *Client) CallAPI(ctx context.Context, method string, rel *url.URL, body, out interface{}) (err error) {
	return c.call(ctx, method, rel, marshaler{
		marshal:   json.Marshal,
		unmarshal: json.Unmarshal,
	}, body, out)
}

// Call executes the desired REST API request.
// Deprecated: use instead client.CallAPI
func (c *Client) Call(ctx context.Context, method string, rel *url.URL, body, out interface{}) (err error) {
	if c.logger != nil {
		c.logger.Warnf(
			"client.Call is deprecated and will be removed soon, please migrate to a client.CallAPI for %s request to '%s'",
			method,
			rel.String(),
		)
	}

	return c.call(ctx, method, rel, marshaler{
		marshal:   jsonapi.Marshal,
		unmarshal: jsonapi.Unmarshal,
	}, body, out)
}

// marshaler is the temporary struct that keeps the marshal/unmarshal methods
type marshaler struct {
	marshal   func(interface{}) ([]byte, error)
	unmarshal func([]byte, interface{}) error
}

func (c *Client) call(
	ctx context.Context,
	method string,
	rel *url.URL,
	marshaler marshaler,
	body, out interface{},
) (err error) {
	if c.logger != nil {
		c.logger.Debugf("[REST API] Making a %s request to '%s'", method, rel.String())
		defer func() {
			if err != nil {
				c.logger.WithError(err).Error("[REST API] Error")
			}
		}()
	}

	var bodyReader io.ReadCloser
	if body != nil {
		var bodyData []byte
		switch val := body.(type) {
		case []byte:
			bodyData = val
		case string:
			bodyData = []byte(val)
		default:
			bodyData, err = marshaler.marshal(body)
			if err != nil {
				return err
			}
		}
		bodyReader = ioutil.NopCloser(bytes.NewBuffer(bodyData))
	}

	req := &http.Request{
		Method: method,
		URL:    c.BaseURL.ResolveReference(rel),
		Body:   bodyReader,
	}
	req = req.WithContext(ctx)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		var errs v1.ErrorResponse
		if err := json.Unmarshal(data, &errs); err != nil {
			return err
		}
		return errs.Errors[0]
	}

	if out != nil {
		return marshaler.unmarshal(data, out)
	}
	return nil
}
