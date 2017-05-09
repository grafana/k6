package cloud

import (
	"fmt"
	"time"

	"github.com/loadimpact/k6/stats"
	"github.com/pkg/errors"
)

type sample struct {
	Type   string     `json:"type"`
	Metric string     `json:"metric"`
	Data   sampleData `json:"data"`
}

type sampleData struct {
	Type  stats.MetricType  `json:"type"`
	Time  time.Time         `json:"time"`
	Value float64           `json:"value"`
	Tags  map[string]string `json:"tags,omitempty"`
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

func (c *Client) CreateTestRun(testRun *TestRun) (*CreateTestRunResponse, error) {
	url := fmt.Sprintf("%s/tests", c.baseURL)
	req, err := c.NewRequest("POST", url, testRun)
	if err != nil {
		return nil, err
	}

	var ctrr = CreateTestRunResponse{}
	err = c.Do(req, &ctrr)
	if err != nil {
		return nil, err
	}

	if ctrr.ReferenceID == "" {
		return nil, errors.Errorf("Failed to get a reference ID")
	}

	return &ctrr, nil
}

func (c *Client) PushMetric(referenceID string, samples []*sample) error {
	url := fmt.Sprintf("%s/metrics/%s", c.baseURL, referenceID)

	req, err := c.NewRequest("POST", url, samples)
	if err != nil {
		return err
	}

	err = c.Do(req, nil)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) TestFinished(referenceID string, thresholds ThresholdResult, tained bool) error {
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
		return err
	}

	err = c.Do(req, nil)
	if err != nil {
		return err
	}

	return nil
}
