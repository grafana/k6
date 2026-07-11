package tests

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cloudapiv6 "go.k6.io/k6/v2/internal/cloudapi/v6"
	"go.k6.io/k6/v2/internal/cloudapi/v6/v6test"
	"go.k6.io/k6/v2/internal/cmd"
)

func TestCloudLoadZoneList(t *testing.T) {
	t.Parallel()

	t.Run("lists load zones successfully", func(t *testing.T) {
		t.Parallel()

		ts := newCloudLoadZoneListTestState(t, testLoadZones())

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, fmt.Sprintf("Load zones for stack-%d:", validStackID))
		assert.Contains(t, stdout, "ID                  NAME                 TYPE      AVAILABLE")
		assert.Contains(t, stdout, "amazon:us:ashburn   US East (Ashburn)    public    yes")
		assert.Contains(t, stdout, "my-cluster          My private cluster   private   no")
	})

	t.Run("empty load zone list", func(t *testing.T) {
		t.Parallel()

		ts := newCloudLoadZoneListTestState(t, nil)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, fmt.Sprintf("Load zones for stack-%d:", validStackID))
		assert.Contains(t, stdout, "No load zones found.")
	})

	t.Run("--help includes usage", func(t *testing.T) {
		t.Parallel()

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{"k6", "cloud", "load-zone", "list", "--help"}

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, "Usage:\n  k6 cloud load-zone list [flags]")
		assert.Contains(t, stdout, "--json")
		assert.NotContains(t, stdout, "Global Flags:")
		assert.NotContains(t, stdout, "--config")
		assert.NotContains(t, stdout, "\n\n\nExamples:")
	})

	t.Run("--json outputs JSON", func(t *testing.T) {
		t.Parallel()

		ts := newCloudLoadZoneListTestState(t, testLoadZones(), "--json")

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()

		var loadZones []cloudapiv6.LoadZone
		require.NoError(t, json.Unmarshal([]byte(stdout), &loadZones))
		assert.Equal(t, testLoadZones(), loadZones)
	})

	t.Run("--json with empty list outputs empty array", func(t *testing.T) {
		t.Parallel()

		ts := newCloudLoadZoneListTestState(t, nil, "--json")

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()

		var loadZones []cloudapiv6.LoadZone
		require.NoError(t, json.Unmarshal([]byte(stdout), &loadZones))
		assert.Empty(t, loadZones)
	})
}

func testLoadZones() []cloudapiv6.LoadZone {
	return []cloudapiv6.LoadZone{
		{ID: 1, K6LoadZoneID: "amazon:us:ashburn", Name: "US East (Ashburn)", Public: true, Available: true},
		{ID: 2, K6LoadZoneID: "my-cluster", Name: "My private cluster", Public: false, Available: false},
	}
}

func newCloudLoadZoneListTestState(
	t *testing.T, loadZones []cloudapiv6.LoadZone, args ...string,
) *GlobalTestState {
	t.Helper()

	srv := v6test.NewServer(t, v6test.Config{LoadZones: loadZones})

	ts := NewGlobalTestState(t)
	ts.CmdArgs = append([]string{"k6", "cloud", "load-zone", "list"}, args...)
	ts.Env["K6_CLOUD_TOKEN"] = validToken
	ts.Env["K6_CLOUD_STACK_ID"] = fmt.Sprintf("%d", validStackID)
	ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

	return ts
}
