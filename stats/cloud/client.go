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
	"github.com/loadimpact/k6/stats"
)

// Client handles communication with Load Impact cloud API.
type Client struct {
	client  *http.Client
	token   string
	baseURL string
}

type ErrorResponse struct {
	Message string `json:"message"`
}

func NewClient(token string) *Client {

	var client = &http.Client{
		Timeout: 30 * time.Second,
	}

	host := os.Getenv("K6CLOUD_HOST")
	if host == "" {
		host = "http://localhost:5000"
	}

	baseURL := fmt.Sprintf("%s/v1", host)

	c := &Client{
		client:  client,
		token:   token,
		baseURL: baseURL,
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

	if v != nil {
		err = json.NewDecoder(resp.Body).Decode(v)
		if err == io.EOF {
			err = nil // Ignore EOF from empty body
		}
	}

	return err
}

type ThresholdResult map[string]map[string]bool

type TestRun struct {
	Name       string              `json:"name"`
	ProjectID  int                 `json:"project_id,omitempty"`
	Thresholds map[string][]string `json:"thresholds"`
	// Duration of test in seconds. -1 for unknown length, 0 for continuous running.
	Duration int64 `json:"duration"`
}

type CreateTestRunResponse struct {
	ReferenceID string `json:"reference_id"`
}

func (c *Client) CreateTestRun(testRun *TestRun) *CreateTestRunResponse {
	url := fmt.Sprintf("%s/tests", c.baseURL)
	req, err := c.NewRequest("POST", url, testRun)
	if err != nil {
		return nil
	}

	var ctrr = CreateTestRunResponse{}
	err = c.Do(req, &ctrr)
	if err != nil {
		return nil
	}

	return &ctrr
}

func (c *Client) PushMetric(referenceID string, samples []*Sample) {
	url := fmt.Sprintf("%s/metrics/%s", c.baseURL, referenceID)

	req, err := c.NewRequest("POST", url, samples)
	if err != nil {
		return
	}

	err = c.Do(req, nil)
	if err != nil {
		return
	}
}

func (c *Client) TestFinished(referenceID string, thresholds ThresholdResult, tained bool) {
	url := fmt.Sprintf("%s/tests/%s", c.baseURL, referenceID)

	status := 1

	if tained {
		status = 2
	}

	data := struct {
		Status     int             `json:"status"`
		Thresholds ThresholdResult `json:"thresholds"`
	}{
		status,
		thresholds,
	}

	req, err := c.NewRequest("POST", url, data)
	if err != nil {
		return
	}

	err = c.Do(req, nil)
	if err != nil {
		return
	}
}

type Sample struct {
	Type   string     `json:"type"`
	Metric string     `json:"metric"`
	Data   SampleData `json:"data"`
}

type SampleData struct {
	Type  stats.MetricType  `json:"type"`
	Time  time.Time         `json:"time"`
	Value float64           `json:"value"`
	Tags  map[string]string `json:"tags,omitempty"`
}
