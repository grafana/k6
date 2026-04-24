package cloudapi

import (
	"bytes"
	"context"
	"fmt"

	"go.k6.io/k6/v2/lib"
)

// ProvisionParams carries the inputs for the full provisioning flow.
type ProvisionParams struct {
	Name          string
	ProjectID     int32
	MaxVUs        int64
	TotalDuration int64          // seconds
	Options       map[string]any // serialised as JSON object in start_local_execution body
	Archive       *lib.Archive   // nil = no archive upload (--no-archive-upload path)
}

// ProvisionResult carries the outputs from the full provisioning flow.
type ProvisionResult struct {
	TestRunID             int32
	TestRunDetailsPageURL string
	RuntimeConfig         RuntimeConfig
}

// ProvisionLocalExecution runs the full provisioning flow and returns the result.
// It calls CreateOrFindLoadTest, StartLocalExecution, and — when Archive is non-nil
// and an upload URL was returned — UploadArchive followed by WaitForTestRunReady.
// NotifyTestRunCompleted is NOT called here; the caller is responsible.
func (c *Client) ProvisionLocalExecution(ctx context.Context, params ProvisionParams) (*ProvisionResult, error) {
	loadTestID, err := c.CreateOrFindLoadTest(ctx, params.ProjectID, params.Name)
	if err != nil {
		return nil, fmt.Errorf("create or find load test: %w", err)
	}

	// Serialise archive once to get its byte-length for the start_local_execution body.
	// The same buffer is reused for the S3 upload to avoid a second serialisation.
	var (
		archiveSize  *int64
		archiveBytes []byte
	)
	if params.Archive != nil {
		var buf bytes.Buffer
		if err := params.Archive.Write(&buf); err != nil {
			return nil, fmt.Errorf("serialising archive: %w", err)
		}
		sz := int64(buf.Len())
		archiveSize = &sz
		archiveBytes = buf.Bytes()
	}

	sleReq := StartLocalExecutionRequest{
		Options:       params.Options,
		MaxVUs:        params.MaxVUs,
		TotalDuration: params.TotalDuration,
		ArchiveSize:   archiveSize,
	}

	sleResp, err := c.StartLocalExecution(ctx, loadTestID, sleReq)
	if err != nil {
		return nil, fmt.Errorf("start local execution: %w", err)
	}

	testRunID := int32(sleResp.TestRunID) //nolint:gosec

	if params.Archive != nil && sleResp.ArchiveUploadURL != nil {
		if err := c.UploadArchive(ctx, *sleResp.ArchiveUploadURL, archiveBytes); err != nil {
			return nil, fmt.Errorf("upload archive: %w", err)
		}

		if err := c.WaitForTestRunReady(ctx, testRunID, 0); err != nil {
			return nil, fmt.Errorf("wait for test run ready: %w", err)
		}
	}

	return &ProvisionResult{
		TestRunID:             testRunID,
		TestRunDetailsPageURL: sleResp.TestRunDetailsPageURL,
		RuntimeConfig:         sleResp.RuntimeConfig,
	}, nil
}
