/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
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

package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/pkg/errors"
)

const (
	// Default request timeout
	RequestTimeout = 10 * time.Second
	// Retry interval
	RetryInterval = 500 * time.Millisecond
	// Retry attempts
	MaxRetries = 3
)

// Client handles communication with Load Impact cloud API.
type Client struct {
	client  *http.Client
	token   string
	baseURL string
	version string

	retries       int
	retryInterval time.Duration
}

func NewClient(token, host, version string) *Client {
	client := &http.Client{
		Timeout: RequestTimeout,
	}

	hostEnv := os.Getenv("K6CLOUD_HOST")
	if hostEnv != "" {
		host = hostEnv
	}
	if host == "" {
		host = "https://ingest.loadimpact.com"
	}

	baseURL := fmt.Sprintf("%s/v1", host)

	c := &Client{
		client:        client,
		token:         token,
		baseURL:       baseURL,
		version:       version,
		retries:       MaxRetries,
		retryInterval: RetryInterval,
	}
	return c
}

func (c *Client) NewRequest(method, url string, data interface{}) (*http.Request, error) {
	var buf io.Reader

	if data != nil {
		b, err := json.Marshal(&data)
		if err != nil {
			return nil, err
		}

		buf = bytes.NewBuffer(b)
	}

	return http.NewRequest(method, url, buf)
}

func (c *Client) Do(req *http.Request, v interface{}) error {
	var originalBody []byte
	var err error

	if req.Body != nil {
		originalBody, err = ioutil.ReadAll(req.Body)
		if err != nil {
			return err
		}

		if cerr := req.Body.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}

	for i := 1; i <= c.retries; i++ {
		if len(originalBody) > 0 {
			req.Body = ioutil.NopCloser(bytes.NewBuffer(originalBody))
		}

		retry, err := c.do(req, v, i)

		if retry {
			time.Sleep(c.retryInterval)
			continue
		}

		return err
	}

	return err
}

func (c *Client) do(req *http.Request, v interface{}, attempt int) (retry bool, err error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Token %s", c.token))
	req.Header.Set("User-Agent", "k6cloud/"+c.version)

	resp, err := c.client.Do(req)

	defer func() {
		if resp != nil {
			if cerr := resp.Body.Close(); cerr != nil && err == nil {
				err = cerr
			}
		}
	}()

	if shouldRetry(resp, err, attempt, c.retries) {
		return true, err
	}

	if err = checkResponse(resp); err != nil {
		return false, err
	}

	if v != nil {
		if err = json.NewDecoder(resp.Body).Decode(v); err == io.EOF {
			err = nil // Ignore EOF from empty body
		}
	}

	return false, err
}

func checkResponse(r *http.Response) error {
	if r == nil {
		return ErrUnknown
	}

	if c := r.StatusCode; c >= 200 && c <= 299 {
		return nil
	}

	if r.StatusCode == 401 {
		return ErrNotAuthenticated
	} else if r.StatusCode == 403 {
		return ErrNotAuthorized
	}

	// Struct of errors set back from API
	errorStruct := &struct {
		ErrorData struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(errorStruct); err != nil {
		return errors.Wrap(err, "Non-standard API error response")
	}

	errorResponse := &ErrorResponse{
		Response: r,
		Message:  errorStruct.ErrorData.Message,
		Code:     errorStruct.ErrorData.Code,
	}

	return errorResponse
}

func shouldRetry(resp *http.Response, err error, attempt, maxAttempts int) bool {
	if attempt >= maxAttempts {
		return false
	}

	if resp == nil || err != nil {
		return true
	}

	if resp.StatusCode >= 500 || resp.StatusCode == 429 {
		return true
	}

	return false
}
