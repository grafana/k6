// Package v6testing provides shared test fixtures for the v6 cloud API.
package v6testing

import (
	"encoding/json"
	"testing"
	"time"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/stretchr/testify/require"
)

var testEpoch = time.Date(2024, 6, 1, 19, 0, 0, 0, time.UTC) //nolint:gochecknoglobals

// TestRunJSON builds a StartLoadTestResponse JSON string using the generated
// model constructor so fixtures break at compile time if the spec changes.
func TestRunJSON(t testing.TB, id int32, status string, result *string, webAppURL string) string {
	t.Helper()
	m := k6cloud.NewStartLoadTestResponse(
		id, 789, 456,
		*k6cloud.NewNullableString(nil), // started_by
		testEpoch,                       // created
		*k6cloud.NewNullableTime(nil),   // ended
		"",                              // note
		*k6cloud.NewNullableTime(nil),   // retention_expiry
		*k6cloud.NewNullableTestCostApiModel(nil), // cost
		status, // status
		*k6cloud.NewStatusApiModel("created", testEpoch), // status_details
		[]k6cloud.StatusApiModel{},                       // status_history
		[]k6cloud.DistributionZoneApiModel{},             // distribution
		*k6cloud.NewNullableString(result),               // result
		map[string]any{},                                 // result_details
		map[string]any{},                                 // options
		map[string]string{},                              // k6_dependencies
		map[string]string{},                              // k6_versions
		*k6cloud.NewNullableInt32(nil),                   // max_vus
		*k6cloud.NewNullableInt32(nil),                   // max_browser_vus
		*k6cloud.NewNullableInt32(nil),                   // estimated_duration
		0,                                                // execution_duration
		webAppURL,                                        // test_run_details_page_url
	)
	b, err := json.Marshal(m)
	require.NoError(t, err)
	return string(b)
}
