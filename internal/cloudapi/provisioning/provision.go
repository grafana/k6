package provisioning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.k6.io/k6/v2/lib"
)

// ProvisionParams contains the parameters for provisioning a local
// execution test run.
type ProvisionParams struct {
	// Name is the load test name.
	Name string

	// ProjectID is the cloud project ID.
	ProjectID int32

	// MaxVUs is the maximum number of virtual users.
	MaxVUs int64

	// TotalDuration is the total test duration in seconds.
	TotalDuration int64

	// Options is the pre-marshalled lib.Options JSON from cmd.
	Options json.RawMessage

	// Archive is the test archive. Nil when --no-archive-upload is set.
	Archive *lib.Archive

	// PollInterval is the interval between test-run status polls.
	// Zero or negative uses the default interval.
	PollInterval time.Duration
}

// ProvisionResult contains the result of a successful provisioning
// operation.
type ProvisionResult struct {
	// TestRunID is the ID of the provisioned test run.
	TestRunID int32

	// TestRunDetailsPageURL is the URL of the test run details page.
	TestRunDetailsPageURL string

	// RuntimeConfig carries the metrics, secrets, and token
	// configuration returned by the provisioning API.
	RuntimeConfig RuntimeConfig
}

// ProvisionLocalExecution orchestrates the full local-execution
// provisioning flow: CreateOrFindLoadTest → StartLocalExecution →
// optional UploadArchive → WaitForTestRunReady.
func (c *Client) ProvisionLocalExecution(
	ctx context.Context, params ProvisionParams,
) (*ProvisionResult, error) {
	loadTestID, err := c.v6Client.CreateOrFindLoadTest(ctx, params.ProjectID, params.Name)
	if err != nil {
		return nil, fmt.Errorf("create or find load test: %w", err)
	}

	// Serialise archive once to get its byte-length for the
	// start_local_execution body. The same buffer is reused for
	// the S3 upload to avoid a second serialisation.
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

	if params.Archive != nil && sleResp.ArchiveUploadURL != nil {
		if err := c.UploadArchive(ctx, *sleResp.ArchiveUploadURL, archiveBytes); err != nil {
			return nil, fmt.Errorf("upload archive: %w", err)
		}
	}

	// Always poll: the backend may queue the test run regardless of
	// whether an archive was uploaded. archive_size=null signals no
	// archive is expected, so the backend skips waiting for upload and
	// goes straight to validation, but the test run may still be queued
	// before becoming ready.
	if err := c.WaitForTestRunReady(ctx, sleResp.TestRunID, params.PollInterval); err != nil {
		return nil, fmt.Errorf("wait for test run ready: %w", err)
	}

	return &ProvisionResult{
		TestRunID:             sleResp.TestRunID,
		TestRunDetailsPageURL: sleResp.TestRunDetailsPageURL,
		RuntimeConfig:         sleResp.RuntimeConfig,
	}, nil
}
