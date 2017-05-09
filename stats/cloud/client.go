package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
)

const (
	TIMEOUT = 10 * time.Second
)

// Client handles communication with Load Impact cloud API.
type Client struct {
	client  *http.Client
	token   string
	baseURL string
	version string
}

func NewClient(token, host, version string) *Client {

	var client = &http.Client{
		Timeout: TIMEOUT,
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
		client:  client,
		token:   token,
		baseURL: baseURL,
		version: version,
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
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Token %s", c.token))
	req.Header.Set("User-Agent", "k6cloud/"+c.version)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Errorln(err)
		}
	}()

	err = checkResponse(resp)

	if v != nil {
		err = json.NewDecoder(resp.Body).Decode(v)
		if err == io.EOF {
			err = nil // Ignore EOF from empty body
		}
	}

	return err
}

func checkResponse(r *http.Response) error {
	if c := r.StatusCode; c >= 200 && c <= 299 {
		return nil
	}

	if r.StatusCode == 401 {
		return AuthenticateError
	} else if r.StatusCode == 403 {
		return AuthorizeError
	}

	// Struct of errors set back from API
	errorStruct := &struct {
		ErrorData struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}{}

	err := json.NewDecoder(r.Body).Decode(errorStruct)
	if err != nil {
		return errors.Wrap(err, "Non-standard API error response")
	}

	errorResponse := &ErrorResponse{
		Response: r,
		Message:  errorStruct.ErrorData.Message,
		Code:     errorStruct.ErrorData.Code,
	}

	return errorResponse
}
