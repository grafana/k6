package provisioning

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
)

// StartLocalExecutionRequest contains the parameters for starting a local
// execution test run via the provisioning API.
type StartLocalExecutionRequest struct {
	// Options is the pre-marshalled lib.Options JSON produced by cmd.
	// Kept as json.RawMessage to preserve typed precision (custom
	// marshallers on lib.Options work correctly).
	Options json.RawMessage

	// MaxVUs is the maximum number of virtual users during the test.
	MaxVUs int64

	// TotalDuration is the total test duration in seconds.
	TotalDuration int64

	// ArchiveSize is the archive file size in bytes, or nil when
	// --no-archive-upload is set (sent as explicit JSON null).
	ArchiveSize *int64
}

// StartLocalExecutionResponse holds the fields returned by the
// start_local_execution endpoint that downstream consumers need.
type StartLocalExecutionResponse struct {
	TestRunID             int64
	ArchiveUploadURL      *string // nil when no archive upload expected
	RuntimeConfig         RuntimeConfig
	TestRunDetailsPageURL string
}

// RuntimeConfig carries the runtime configuration returned by the
// provisioning API for a local-execution test run.
type RuntimeConfig struct {
	Metrics      MetricsConfig
	TestRunToken string
	Secrets      SecretsConfig
}

// MetricsConfig holds the metrics push configuration from the
// provisioning API's runtime_config.metrics object.
type MetricsConfig struct {
	PushURL               string
	PushInterval          *string
	PushConcurrency       *int32
	AggregationPeriod     *string
	AggregationWaitPeriod *string
	AggregationMinSamples *int32
	MaxSamplesPerPackage  *int32
}

// SecretsConfig holds the secrets configuration from the
// provisioning API's runtime_config.secrets object.
type SecretsConfig struct {
	Endpoint     string
	ResponsePath string
}

// StartLocalExecution starts a local-execution test run via the
// provisioning API. It generates a K6-Idempotency-Key header for
// safe retries. The caller provides options as pre-marshalled JSON.
func (c *Client) StartLocalExecution(
	ctx context.Context, loadTestID int64, req StartLocalExecutionRequest,
) (*StartLocalExecutionResponse, error) {
	// Generate idempotency key: 8 random bytes hex-encoded (16 chars).
	var key [8]byte
	if _, err := rand.Read(key[:]); err != nil {
		return nil, fmt.Errorf("generating idempotency key: %w", err)
	}

	// SDK adapter: unmarshal json.RawMessage → map[string]any.
	var opts map[string]any
	if err := json.Unmarshal(req.Options, &opts); err != nil {
		return nil, fmt.Errorf("unmarshalling options for SDK: %w", err)
	}

	maxVUs, err := toInt32(req.MaxVUs)
	if err != nil {
		return nil, fmt.Errorf("max_vus: %w", err)
	}
	totalDuration, err := toInt32(req.TotalDuration)
	if err != nil {
		return nil, fmt.Errorf("total_duration: %w", err)
	}

	sdkReq := k6cloud.NewStartLocalExecutionTestRequest(opts, maxVUs, totalDuration)

	if req.ArchiveSize != nil {
		v, err := toInt32(*req.ArchiveSize)
		if err != nil {
			return nil, fmt.Errorf("archive_size: %w", err)
		}
		sdkReq.SetArchiveSize(v)
	} else {
		sdkReq.SetArchiveSizeNil()
	}

	res, hr, err := c.apiClient.ProvisioningAPI.
		StartLocalExecutionTest(c.authCtx(ctx), loadTestID).
		K6IdempotencyKey(hex.EncodeToString(key[:])).
		StartLocalExecutionTestRequest(sdkReq).
		Execute()
	defer closeResponse(hr, &err)

	if hr != nil {
		if respErr := CheckResponse(hr); respErr != nil {
			return nil, respErr
		}
	}
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errUnknown
	}

	return mapStartLocalExecutionResponse(res), nil
}

// mapStartLocalExecutionResponse converts the SDK response type into
// the package's domain type.
func mapStartLocalExecutionResponse(res *k6cloud.StartLocalExecutionTestResponse) *StartLocalExecutionResponse {
	rc := res.GetRuntimeConfig()
	m := rc.GetMetrics()
	s := rc.GetSecrets()

	resp := &StartLocalExecutionResponse{
		TestRunID:             res.GetTestRunId(),
		TestRunDetailsPageURL: res.GetTestRunDetailsPageUrl(),
		RuntimeConfig: RuntimeConfig{
			TestRunToken: rc.GetTestRunToken(),
			Metrics: MetricsConfig{
				PushURL:               m.GetPushUrl(),
				PushInterval:          m.PushInterval.Get(),
				PushConcurrency:       m.PushConcurrency.Get(),
				AggregationPeriod:     m.AggregationPeriod.Get(),
				AggregationWaitPeriod: m.AggregationWaitPeriod.Get(),
				AggregationMinSamples: m.AggregationMinSamples.Get(),
				MaxSamplesPerPackage:  m.MaxSamplesPerPackage.Get(),
			},
			Secrets: SecretsConfig{
				Endpoint:     s.GetEndpoint(),
				ResponsePath: s.GetResponsePath(),
			},
		},
	}

	if url := res.ArchiveUploadUrl.Get(); url != nil {
		resp.ArchiveUploadURL = url
	}

	return resp
}

// toInt32 safely converts an int64 to int32, returning an error if
// the value overflows.
func toInt32(v int64) (int32, error) {
	if v < math.MinInt32 || v > math.MaxInt32 {
		return 0, fmt.Errorf("value %d overflows int32", v)
	}
	return int32(v), nil
}

// closeResponse drains and closes an HTTP response body. It mirrors
// the v6 package's closeResponse helper.
func closeResponse(res *http.Response, rerr *error) {
	if res == nil {
		return
	}
	_, _ = io.Copy(io.Discard, res.Body)
	if err := res.Body.Close(); err != nil && *rerr == nil {
		*rerr = err
	}
}
