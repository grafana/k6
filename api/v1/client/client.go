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

	"github.com/loadimpact/k6/api/v1"
)

type Client struct {
	BaseURL *url.URL
}

func New(base string) (*Client, error) {
	baseURL, err := url.Parse("http://" + base)
	if err != nil {
		return nil, err
	}
	return &Client{BaseURL: baseURL}, nil
}

func (c *Client) call(ctx context.Context, method string, rel *url.URL, body, out interface{}) error {
	var bodyReader io.ReadCloser
	if body != nil {
		bodyData, err := jsonapi.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = ioutil.NopCloser(bytes.NewBuffer(bodyData))
	}

	req := &http.Request{
		Method: method,
		URL:    c.BaseURL.ResolveReference(rel),
		Body:   bodyReader,
	}
	req = req.WithContext(ctx)

	res, err := http.DefaultClient.Do(req)
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

	return jsonapi.Unmarshal(data, out)
}
