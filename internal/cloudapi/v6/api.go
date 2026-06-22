package cloudapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"

	"go.k6.io/k6/v2/lib"
)

// Project is a Grafana Cloud k6 project.
type Project struct {
	ID        int32  `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
}

// LoadTest is a Grafana Cloud k6 load test.
type LoadTest struct {
	ID        int32     `json:"id"`
	ProjectID int32     `json:"project_id"`
	Name      string    `json:"name"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
}

// ListProjects retrieves the list of projects for the configured stack.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	const pageSize int32 = 1000

	projects := []Project{}
	var skip int32

	for {
		res, err := c.listProjectsPage(ctx, skip, pageSize)
		if err != nil {
			return nil, err
		}

		for _, project := range res.Value {
			projects = append(projects, Project{
				ID:        project.Id,
				Name:      project.Name,
				IsDefault: project.IsDefault,
			})
		}

		if res.NextLink == nil || *res.NextLink == "" {
			return projects, nil
		}

		if len(res.Value) == 0 {
			return nil, errors.New("received empty projects page with next link")
		}
		skip += pageSize
	}
}

func (c *Client) listProjectsPage(
	ctx context.Context, skip, top int32,
) (*k6cloud.ProjectListResponse, error) {
	res, hr, err := c.apiClient.ProjectsAPI.
		ProjectsList(c.authCtx(ctx)).
		XStackId(c.stackID).
		Skip(skip).
		Top(top).
		Execute()
	defer closeResponse(hr, &err)

	if err := CheckResponse(hr, err); err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errUnknown
	}

	return res, nil
}

// ListLoadTests retrieves the list of load tests in the given project.
func (c *Client) ListLoadTests(ctx context.Context, projectID int32) ([]LoadTest, error) {
	const pageSize int32 = 1000

	tests := []LoadTest{}
	var skip int32

	for {
		res, err := c.listLoadTestsPage(ctx, projectID, skip, pageSize)
		if err != nil {
			return nil, err
		}

		for _, test := range res.Value {
			tests = append(tests, LoadTest{
				ID:        test.Id,
				ProjectID: test.ProjectId,
				Name:      test.Name,
				Created:   test.Created,
				Updated:   test.Updated,
			})
		}

		if res.NextLink == nil || *res.NextLink == "" {
			return tests, nil
		}

		if len(res.Value) == 0 {
			return nil, errors.New("received empty load tests page with next link")
		}
		skip += pageSize
	}
}

func (c *Client) listLoadTestsPage(
	ctx context.Context, projectID, skip, top int32,
) (*k6cloud.LoadTestListResponse, error) {
	res, hr, err := c.apiClient.LoadTestsAPI.
		ProjectsLoadTestsRetrieve(c.authCtx(ctx), projectID).
		XStackId(c.stackID).
		Skip(skip).
		Top(top).
		Execute()
	defer closeResponse(hr, &err)

	if err := CheckResponse(hr, err); err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errUnknown
	}

	return res, nil
}

// ValidateToken validates the cloud authentication token.
func (c *Client) ValidateToken(ctx context.Context, stackURL string) (_ *k6cloud.AuthenticationResponse, err error) {
	if stackURL == "" {
		return nil, errors.New("stack URL is required to validate token")
	}
	if _, err := url.Parse(stackURL); err != nil {
		return nil, fmt.Errorf("invalid stack URL: %w", err)
	}

	res, hr, err := c.apiClient.AuthorizationAPI.
		Auth(c.authCtx(ctx)).
		XStackUrl(stackURL).
		Execute()
	defer closeResponse(hr, &err)

	if err := CheckResponse(hr, err); err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errUnknown
	}

	return res, nil
}

// ValidateOptions validates cloud test options.
func (c *Client) ValidateOptions(ctx context.Context, projectID int32, opts lib.Options) (err error) {
	// Round-trip [lib.Options] through JSON so every script option
	// reaches the backend via [k6cloud.Options.AdditionalProperties].
	buf, err := json.Marshal(opts)
	if err != nil {
		return err
	}
	copts := *k6cloud.NewOptions()
	if err := json.Unmarshal(buf, &copts.AdditionalProperties); err != nil {
		return err
	}

	req := k6cloud.NewValidateOptionsRequest(copts)
	if projectID > 0 {
		req.SetProjectId(projectID)
	}
	res, hr, err := c.apiClient.LoadTestsAPI.
		ValidateOptions(c.authCtx(ctx)).
		XStackId(c.stackID).
		ValidateOptionsRequest(req).
		Execute()
	defer closeResponse(hr, &err)

	if err := CheckResponse(hr, err); err != nil {
		return err
	}
	if res == nil {
		return errUnknown
	}

	return nil
}

// UploadTest creates or updates a cloud load test's script.
func (c *Client) UploadTest(
	ctx context.Context, name string, projectID int32, arc *lib.Archive,
) (*k6cloud.LoadTestApiModel, error) {
	lt, err := c.createTest(ctx, name, projectID, arc)
	if err == nil {
		return lt, nil
	}
	var rerr ResponseError
	if !errors.As(err, &rerr) || rerr.Response == nil || rerr.Response.StatusCode != http.StatusConflict {
		return nil, err
	}

	// 409: a test with this name already exists in this project. Look it
	// up by exact-match filter and update its script.
	lt, err = c.findTestByName(ctx, projectID, name)
	if err != nil {
		return nil, err
	}
	if err := c.updateScript(ctx, lt.GetId(), arc); err != nil {
		return nil, err
	}

	return lt, nil
}

// createTest creates a new cloud load test in the given project.
func (c *Client) createTest(
	ctx context.Context, name string, projectID int32, arc *lib.Archive,
) (_ *k6cloud.LoadTestApiModel, err error) {
	res, hr, err := c.apiClient.LoadTestsAPI.
		ProjectsLoadTestsCreate(c.authCtx(ctx), projectID).
		XStackId(c.stackID).
		Name(name).
		Script(archiveReader(arc)).
		Execute()
	defer closeResponse(hr, &err)

	if err := CheckResponse(hr, err); err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errUnknown
	}

	return res, nil
}

func (c *Client) findTestByName(
	ctx context.Context, projectID int32, name string,
) (_ *k6cloud.LoadTestApiModel, err error) {
	res, hr, err := c.apiClient.LoadTestsAPI.
		ProjectsLoadTestsRetrieve(c.authCtx(ctx), projectID).
		XStackId(c.stackID).
		Name(name).
		Top(1).
		Execute()
	defer closeResponse(hr, &err)

	if err := CheckResponse(hr, err); err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errUnknown
	}

	tests := res.GetValue()
	if len(tests) == 0 {
		return nil, errTestNotExists
	}

	return &tests[0], nil
}

func (c *Client) updateScript(ctx context.Context, testID int32, arc *lib.Archive) (err error) {
	res, err := c.apiClient.LoadTestsAPI.
		LoadTestsScriptUpdate(c.authCtx(ctx), testID).
		XStackId(c.stackID).
		Body(archiveReader(arc)).
		Execute()
	defer closeResponse(res, &err)

	return CheckResponse(res, err)
}

// StartTest starts a cloud load test run.
func (c *Client) StartTest(ctx context.Context, loadTestID int32) (_ *k6cloud.StartLoadTestResponse, err error) {
	var key [8]byte
	if _, err := rand.Read(key[:]); err != nil {
		return nil, err
	}

	res, hr, err := c.apiClient.LoadTestsAPI.
		LoadTestsStart(c.authCtx(ctx), loadTestID).
		XStackId(c.stackID).
		K6IdempotencyKey(hex.EncodeToString(key[:])).
		Execute()
	defer closeResponse(hr, &err)

	if err := CheckResponse(hr, err); err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errUnknown
	}

	return res, nil
}

// StopTest aborts a running cloud test run.
func (c *Client) StopTest(ctx context.Context, testRunID int32) (err error) {
	hr, err := c.apiClient.TestRunsAPI.
		TestRunsAbort(c.authCtx(ctx), testRunID).
		XStackId(c.stackID).
		Execute()
	defer closeResponse(hr, &err)

	err = CheckResponse(hr, err)
	var rerr ResponseError
	if errors.As(err, &rerr) && rerr.Response != nil && rerr.Response.StatusCode == http.StatusConflict {
		return nil // Already stopped: swallow the error to keep the caller/TUI clean.
	}

	return err
}

// FetchTest fetches the current progress of a cloud test run.
func (c *Client) FetchTest(ctx context.Context, testRunID int32) (_ *TestProgress, err error) {
	res, hr, err := c.apiClient.TestRunsAPI.
		TestRunsRetrieve(c.authCtx(ctx), testRunID).
		XStackId(c.stackID).
		Execute()
	defer closeResponse(hr, &err)

	if err := CheckResponse(hr, err); err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errUnknown
	}

	return &TestProgress{
		Status:            Status(res.GetStatus()),
		Result:            Result(res.GetResult()),
		EstimatedDuration: res.GetEstimatedDuration(),
		ExecutionDuration: res.GetExecutionDuration(),
		StatusHistory:     FromStatusModel(res.GetStatusHistory()),
	}, nil
}

func (c *Client) authCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, k6cloud.ContextAccessToken, c.token)
}

func closeResponse(res *http.Response, rerr *error) {
	if res == nil {
		return
	}
	_, _ = io.Copy(io.Discard, res.Body)
	if err := res.Body.Close(); err != nil && *rerr == nil {
		*rerr = err
	}
}

func archiveReader(arc *lib.Archive) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		pw.CloseWithError(arc.Write(pw))
	}()
	return pr
}
