package cloudapi

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/lib"
)

type ResultStatus int

const (
	ResultStatusPassed ResultStatus = 0
	ResultStatusFailed ResultStatus = 1
)

type ThresholdResult map[string]map[string]bool

type TestRun struct {
	Name       string              `json:"name"`
	ProjectID  int64               `json:"project_id,omitempty"`
	VUsMax     int64               `json:"vus"`
	Thresholds map[string][]string `json:"thresholds"`
	// Duration of test in seconds. -1 for unknown length, 0 for continuous running.
	Duration int64 `json:"duration"`
}

// LogEntry can be used by the cloud to tell k6 to log something to the console,
// so the user can see it.
type LogEntry struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

type CreateTestRunResponse struct {
	ReferenceID    string     `json:"reference_id"`
	ConfigOverride *Config    `json:"config"`
	Logs           []LogEntry `json:"logs"`
}

type TestProgressResponse struct {
	RunStatusText string       `json:"run_status_text"`
	RunStatus     RunStatus    `json:"run_status"`
	ResultStatus  ResultStatus `json:"result_status"`
	Progress      float64      `json:"progress"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

func (c *Client) handleLogEntriesFromCloud(ctrr CreateTestRunResponse) {
	logger := c.logger.WithField("source", "grafana-k6-cloud")
	for _, logEntry := range ctrr.Logs {
		level, err := logrus.ParseLevel(logEntry.Level)
		if err != nil {
			logger.Debugf("invalid message level '%s' for message '%s': %s", logEntry.Level, logEntry.Message, err)
			level = logrus.ErrorLevel
		}
		logger.Log(level, logEntry.Message)
	}
}

// CreateTestRun is used when a test run is being executed locally, while the
// results are streamed to the cloud, i.e. `k6 run --out cloud script.js`.
func (c *Client) CreateTestRun(testRun *TestRun) (*CreateTestRunResponse, error) {
	url := fmt.Sprintf("%s/tests", c.baseURL)
	req, err := c.NewRequest("POST", url, testRun)
	if err != nil {
		return nil, err
	}

	ctrr := CreateTestRunResponse{}
	err = c.Do(req, &ctrr)
	if err != nil {
		return nil, err
	}

	c.handleLogEntriesFromCloud(ctrr)
	if ctrr.ReferenceID == "" {
		return nil, fmt.Errorf("failed to get a reference ID")
	}

	return &ctrr, nil
}

// StartCloudTestRun starts a cloud test run, i.e. `k6 cloud script.js`.
func (c *Client) StartCloudTestRun(name string, projectID int64, arc *lib.Archive) (*CreateTestRunResponse, error) {
	requestUrl := fmt.Sprintf("%s/archive-upload", c.baseURL)

	var buf bytes.Buffer
	mp := multipart.NewWriter(&buf)

	if err := mp.WriteField("name", name); err != nil {
		return nil, err
	}

	if projectID != 0 {
		if err := mp.WriteField("project_id", strconv.FormatInt(projectID, 10)); err != nil {
			return nil, err
		}
	}

	fw, err := mp.CreateFormFile("file", "archive.tar")
	if err != nil {
		return nil, err
	}

	if err := arc.Write(fw); err != nil {
		return nil, err
	}

	if err := mp.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", requestUrl, &buf)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", mp.FormDataContentType())

	ctrr := CreateTestRunResponse{}
	if err := c.Do(req, &ctrr); err != nil {
		return nil, err
	}
	c.handleLogEntriesFromCloud(ctrr)
	return &ctrr, nil
}

// TestFinished sends the result and run status values to the cloud, along with
// information for the test thresholds, and marks the test run as finished.
func (c *Client) TestFinished(referenceID string, thresholds ThresholdResult, tained bool, runStatus RunStatus) error {
	url := fmt.Sprintf("%s/tests/%s", c.baseURL, referenceID)

	resultStatus := ResultStatusPassed
	if tained {
		resultStatus = ResultStatusFailed
	}

	data := struct {
		ResultStatus ResultStatus    `json:"result_status"`
		RunStatus    RunStatus       `json:"run_status"`
		Thresholds   ThresholdResult `json:"thresholds"`
	}{
		resultStatus,
		runStatus,
		thresholds,
	}

	req, err := c.NewRequest("POST", url, data)
	if err != nil {
		return err
	}

	return c.Do(req, nil)
}

func (c *Client) GetTestProgress(referenceID string) (*TestProgressResponse, error) {
	url := fmt.Sprintf("%s/test-progress/%s", c.baseURL, referenceID)
	req, err := c.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	ctrr := TestProgressResponse{}
	err = c.Do(req, &ctrr)
	if err != nil {
		return nil, err
	}

	return &ctrr, nil
}

func (c *Client) StopCloudTestRun(referenceID string) error {
	url := fmt.Sprintf("%s/tests/%s/stop", c.baseURL, referenceID)

	req, err := c.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

	return c.Do(req, nil)
}

func (c *Client) ValidateOptions(options lib.Options) error {
	url := fmt.Sprintf("%s/validate-options", c.baseURL)

	data := struct {
		Options lib.Options `json:"options"`
	}{
		options,
	}

	req, err := c.NewRequest("POST", url, data)
	if err != nil {
		return err
	}

	return c.Do(req, nil)
}

func (c *Client) Login(email string, password string) (*LoginResponse, error) {
	url := fmt.Sprintf("%s/login", c.baseURL)

	data := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{
		email,
		password,
	}

	req, err := c.NewRequest("POST", url, data)
	if err != nil {
		return nil, err
	}

	lr := LoginResponse{}
	err = c.Do(req, &lr)
	if err != nil {
		return nil, err
	}

	return &lr, nil
}
