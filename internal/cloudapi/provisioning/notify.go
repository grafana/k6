package provisioning

import (
	"context"
	"errors"
	"fmt"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"

	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/internal/cloudapi/httputil"
)

// notifyError represents a script execution error in the notify
// request body, mapping k6 abort reasons to canonical k6-cloud codes.
type notifyError struct {
	Code   int32
	Reason string
}

// mapTestErrorToNotifyCode translates a k6 test error (with an
// optional errext.AbortReason) into the canonical k6-cloud notify
// error code. Returns nil when the test completed without error or
// when the abort reason indicates a normal completion
// (AbortedByThresholdsAfterTestEnd).
func mapTestErrorToNotifyCode(testErr error) *notifyError {
	if testErr == nil {
		return nil
	}

	var hasAbortReason errext.HasAbortReason
	if errors.As(testErr, &hasAbortReason) {
		switch hasAbortReason.AbortReason() {
		case errext.AbortedByUser, errext.AbortedByScriptAbort:
			return &notifyError{Code: 8036, Reason: testErr.Error()}
		case errext.AbortedByThreshold:
			return &notifyError{Code: 8036, Reason: testErr.Error()}
		case errext.AbortedByThresholdsAfterTestEnd:
			return nil
		case errext.AbortedByScriptError:
			return &notifyError{Code: 8035, Reason: testErr.Error()}
		case errext.AbortedByTimeout, errext.AbortedByOutput:
			return &notifyError{Code: 8034, Reason: testErr.Error()}
		}
	}

	return &notifyError{Code: 0, Reason: testErr.Error()}
}

// NotifyTestRunCompleted posts a script_execution_completed event to
// POST /provisioning/v1/test_runs/{id}/notify using the generated SDK
// method. The scoped testRunToken is passed via context (not the
// long-lived API token). testErr is mapped to an optional error code
// in the request body. Retries on 5xx via the SDK's built-in retry;
// does not retry on 4xx.
func (c *Client) NotifyTestRunCompleted(
	ctx context.Context, testRunID int32, testRunToken string, testErr error,
) error {
	// Use the scoped test-run token instead of the long-lived API token.
	// The SDK reads ContextAccessToken from the context and sets
	// Authorization: Bearer <token>.
	scopedCtx := context.WithValue(ctx, k6cloud.ContextAccessToken, testRunToken)

	model := k6cloud.NewScriptExecutionCompletedNotificationApiModel("script_execution_completed")
	if ne := mapTestErrorToNotifyCode(testErr); ne != nil {
		model.SetError(*k6cloud.NewScriptExecutionError(ne.Code, ne.Reason))
	} else {
		model.SetErrorNil()
	}

	hr, err := c.apiClient.ProvisioningAPI.
		TestRunsNotify(scopedCtx, testRunID).
		ScriptExecutionCompletedNotificationApiModel(model).
		Execute() //nolint:bodyclose // response body is drained and closed via httputil.CloseResponse below
	defer httputil.CloseResponse(hr, &err)

	if hr != nil {
		if respErr := CheckResponse(hr); respErr != nil {
			return fmt.Errorf("notify test run completed: %w", respErr)
		}
	}
	if err != nil {
		return fmt.Errorf("notify test run completed: %w", err)
	}

	return nil
}
