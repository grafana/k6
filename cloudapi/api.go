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

// ResultStatus represents the result status of a test.
type ResultStatus int

const (
	// ResultStatusPassed means the test has passed
	ResultStatusPassed ResultStatus = 0
	// ResultStatusFailed means the test has failed
	ResultStatusFailed ResultStatus = 1
)

// ThresholdResult is a helper type to make sending the thresholds result to the cloud.
type ThresholdResult map[string]map[string]bool

// TestRun represents a test run.
type TestRun struct {
	Name       string              `json:"name"`
	ProjectID  int64               `json:"project_id,omitempty"`
	VUsMax     int64               `json:"vus"`
	Thresholds map[string][]string `json:"thresholds"`
	// Duration of test in seconds. -1 for unknown length, 0 for continuous running.
	Duration int64 `json:"duration"`
	// Archive is the test script archive to (maybe) upload to the cloud.
	Archive *lib.Archive `json:"-"`
}

// LogEntry can be used by the cloud to tell k6 to log something to the console,
// so the user can see it.
type LogEntry struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

// CreateTestRunResponse represents the response of successfully created test run in the cloud.
type CreateTestRunResponse struct {
	ReferenceID    string     `json:"reference_id"`
	ConfigOverride *Config    `json:"config"`
	Logs           []LogEntry `json:"logs"`
}

// TestProgressResponse represents the progress of a cloud test.
type TestProgressResponse struct {
	RunStatusText string       `json:"run_status_text"`
	RunStatus     RunStatus    `json:"run_status"`
	ResultStatus  ResultStatus `json:"result_status"`
	Progress      float64      `json:"progress"`
}

// LoginResponse includes the token after a successful login.
type LoginResponse struct {
	Token string `json:"token"`
}

// ValidateTokenResponse is the response of a token validation.
type ValidateTokenResponse struct {
	IsValid bool   `json:"is_valid"`
	Message string `json:"message"`
	Token   string `json:"token-info"`
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
// results are streamed to the cloud, i.e. `k6 cloud run --local-execution` or `k6 run --out cloud script.js`.
func (c *Client) CreateTestRun(testRun *TestRun) (*CreateTestRunResponse, error) {
	url := fmt.Sprintf("%s/tests", c.baseURL)

	// Because the kind of request we make can vary depending on the test run configuration, we delegate
	// its creation to a helper.
	request, err := c.makeCreateTestRunRequest(url, testRun)
	if err != nil {
		return nil, err
	}

	response := CreateTestRunResponse{}
	err = c.Do(request, &response)
	if err != nil {
		return nil, err
	}

	c.handleLogEntriesFromCloud(response)
	if response.ReferenceID == "" {
		return nil, fmt.Errorf("failed to get a reference ID")
	}

	return &response, nil
}

// makeCreateTestRunRequest creates a new HTTP request for creating a test run.
//
// If the test run archive isn't set, the request will be a regular JSON request with the test run information.
// Otherwise, the request will be a multipart form request containing the test run information and the archive file.
func (c *Client) makeCreateTestRunRequest(url string, testRun *TestRun) (*http.Request, error) {
	// If the test run archive isn't set, we are not uploading an archive and can use the regular request JSON format.
	if testRun.Archive == nil {
		return c.NewRequest(http.MethodPost, url, testRun)
	}

	// Otherwise, we need to create a multipart form request containing the test run information as
	// well as the archive file.
	fields := [][2]string{
		{"name", testRun.Name},
		{"project_id", strconv.FormatInt(testRun.ProjectID, 10)},
		{"vus", strconv.FormatInt(testRun.VUsMax, 10)},
		{"duration", strconv.FormatInt(testRun.Duration, 10)},
	}

	var buffer bytes.Buffer
	multipartWriter := multipart.NewWriter(&buffer)

	for _, field := range fields {
		if err := multipartWriter.WriteField(field[0], field[1]); err != nil {
			return nil, err
		}
	}

	fw, err := multipartWriter.CreateFormFile("file", "archive.tar")
	if err != nil {
		return nil, err
	}

	if err = testRun.Archive.Write(fw); err != nil {
		return nil, err
	}

	// Close the multipart writer to finalize the form data
	err = multipartWriter.Close()
	if err != nil {
		return nil, err
	}

	// Create a new POST request with the multipart form data
	req, err := http.NewRequest(http.MethodPost, url, &buffer) //nolint:noctx
	if err != nil {
		return nil, err
	}

	// Set the content type to the one generated by the multipart writer
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	return req, nil
}

// StartCloudTestRun starts a cloud test run, i.e. `k6 cloud script.js`.
func (c *Client) StartCloudTestRun(name string, projectID int64, arc *lib.Archive) (*CreateTestRunResponse, error) {
	fields := [][2]string{{"name", name}}

	if projectID != 0 {
		fields = append(fields, [2]string{"project_id", strconv.FormatInt(projectID, 10)})
	}

	return c.uploadArchive(fields, arc)
}

// UploadTestOnly uploads a test run to the cloud without actually starting it.
func (c *Client) UploadTestOnly(name string, projectID int64, arc *lib.Archive) (*CreateTestRunResponse, error) {
	fields := [][2]string{{"name", name}, {"upload_only", "true"}}

	if projectID != 0 {
		fields = append(fields, [2]string{"project_id", strconv.FormatInt(projectID, 10)})
	}

	return c.uploadArchive(fields, arc)
}

func (c *Client) uploadArchive(fields [][2]string, arc *lib.Archive) (*CreateTestRunResponse, error) {
	requestURL := fmt.Sprintf("%s/archive-upload", c.baseURL)

	var buf bytes.Buffer
	mp := multipart.NewWriter(&buf)

	for _, field := range fields {
		if err := mp.WriteField(field[0], field[1]); err != nil {
			return nil, err
		}
	}

	fw, err := mp.CreateFormFile("file", "archive.tar")
	if err != nil {
		return nil, err
	}

	if err = arc.Write(fw); err != nil {
		return nil, err
	}

	if err = mp.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, requestURL, &buf) //nolint:noctx
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

// GetTestProgress for the provided referenceID.
func (c *Client) GetTestProgress(referenceID string) (*TestProgressResponse, error) {
	req, err := c.NewRequest(http.MethodGet, c.baseURL+"/test-progress/"+referenceID, nil)
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

// StopCloudTestRun tells the cloud to stop the test with the provided referenceID.
func (c *Client) StopCloudTestRun(referenceID string) error {
	req, err := c.NewRequest("POST", c.baseURL+"/tests/"+referenceID+"/stop", nil)
	if err != nil {
		return err
	}

	return c.Do(req, nil)
}

type validateOptionsRequest struct {
	Options lib.Options `json:"options"`
}

// ValidateOptions sends the provided options to the cloud for validation.
func (c *Client) ValidateOptions(options lib.Options) error {
	data := validateOptionsRequest{Options: options}
	req, err := c.NewRequest("POST", c.baseURL+"/validate-options", data)
	if err != nil {
		return err
	}

	return c.Do(req, nil)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login the user with the specified email and password.
func (c *Client) Login(email string, password string) (*LoginResponse, error) {
	data := loginRequest{Email: email, Password: password}
	req, err := c.NewRequest("POST", c.baseURL+"/login", data)
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

type validateTokenRequest struct {
	Token string `json:"token"`
}

// ValidateToken calls the endpoint to validate the Client's token and returns the result.
func (c *Client) ValidateToken() (*ValidateTokenResponse, error) {
	data := validateTokenRequest{Token: c.token}
	req, err := c.NewRequest("POST", c.baseURL+"/validate-token", data)
	if err != nil {
		return nil, err
	}

	vtr := ValidateTokenResponse{}
	err = c.Do(req, &vtr)
	if err != nil {
		return nil, err
	}

	return &vtr, nil
}
