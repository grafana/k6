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
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// RequestTimeout is the default cloud request timeout
	RequestTimeout = 20 * time.Second
	// RetryInterval is the default cloud request retry interval
	RetryInterval = 500 * time.Millisecond
	// MaxRetries specifies max retry attempts
	MaxRetries = 3

	k6IdempotencyKeyHeader = "k6-Idempotency-Key"
)

// Client handles communication with Load Impact cloud API.
type Client struct {
	client         *http.Client
	token          string
	baseURL        string
	version        string
	pushBufferPool sync.Pool
	logger         logrus.FieldLogger

	retries       int
	retryInterval time.Duration
}

// NewClient return a new client for the cloud API
func NewClient(logger logrus.FieldLogger, token, host, version string) *Client {
	c := &Client{
		client:        &http.Client{Timeout: RequestTimeout},
		token:         token,
		baseURL:       fmt.Sprintf("%s/v1", host),
		version:       version,
		retries:       MaxRetries,
		retryInterval: RetryInterval,
		pushBufferPool: sync.Pool{
			New: func() interface{} {
				return &bytes.Buffer{}
			},
		},
		logger: logger,
	}
	return c
}

// NewRequest creates new HTTP request.
//
// This is the same as http.NewRequest, except that data if not nil
// will be serialized in json format.
func (c *Client) NewRequest(method, url string, data interface{}) (*http.Request, error) {
	var buf io.Reader

	if data != nil {
		b, err := json.Marshal(&data)
		if err != nil {
			return nil, err
		}

		buf = bytes.NewBuffer(b)
	}

	req, err := http.NewRequest(method, url, buf)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (c *Client) Do(req *http.Request, v interface{}) error {
	if req.Body != nil && req.GetBody == nil {
		originalBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return err
		}
		if err = req.Body.Close(); err != nil {
			return err
		}

		req.GetBody = func() (io.ReadCloser, error) {
			return ioutil.NopCloser(bytes.NewReader(originalBody)), nil
		}
		req.Body, _ = req.GetBody()
	}

	// TODO(cuonglm): finding away to move this back to NewRequest
	c.prepareHeaders(req)

	for i := 1; i <= c.retries; i++ {
		retry, err := c.do(req, v, i)

		if retry {
			time.Sleep(c.retryInterval)
			if req.GetBody != nil {
				req.Body, _ = req.GetBody()
			}
			continue
		}

		return err
	}

	return nil
}

func (c *Client) prepareHeaders(req *http.Request) {
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Token %s", c.token))
	}

	if shouldAddIdempotencyKey(req) {
		req.Header.Set(k6IdempotencyKeyHeader, randomStrHex())
	}

	req.Header.Set("User-Agent", "k6cloud/"+c.version)
}

func (c *Client) do(req *http.Request, v interface{}, attempt int) (retry bool, err error) {
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

	if err != nil {
		return false, err
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

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	var payload struct {
		Error ErrorResponse `json:"error"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		if r.StatusCode == http.StatusUnauthorized {
			return ErrNotAuthenticated
		}
		if r.StatusCode == http.StatusForbidden {
			return ErrNotAuthorized
		}
		return errors.Errorf(
			"Unexpected HTTP error from %s: %d %s",
			r.Request.URL,
			r.StatusCode,
			http.StatusText(r.StatusCode),
		)
	}
	payload.Error.Response = r
	return payload.Error
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

func shouldAddIdempotencyKey(req *http.Request) bool {
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return req.Header.Get(k6IdempotencyKeyHeader) == ""
	}
}

// randomStrHex returns a hex string which can be used
// for session token id or idempotency key.
//nolint:gosec
func randomStrHex() string {
	// 16 hex characters
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}
