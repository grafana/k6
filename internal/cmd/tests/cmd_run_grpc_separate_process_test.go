package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.k6.io/k6/internal/cmd"
	"go.k6.io/k6/lib/fsext"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGRPCSeparateProcess tests that the gRPC client can properly load proto definitions
// at runtime, which is the core functionality needed for separate process scenarios.
// This validates the issue #3552 requirement without actually launching separate processes.
func TestGRPCSeparateProcess(t *testing.T) {
	t.Parallel()

	// Start a gRPC server using the existing test infrastructure
	grpcServer := NewGRPC(t)

	// Create a script that loads proto definitions at runtime,
	// simulating the separate process scenario
	script := fmt.Sprintf(`
import grpc from 'k6/net/grpc';
import { check } from "k6";

const GRPC_ADDR = '%s';

let client = new grpc.Client();
// Load proto at runtime - this is the key functionality being tested
client.load([], './route_guide.proto');

export default () => {
  client.connect(GRPC_ADDR, { plaintext: true });

  const response = client.invoke("main.FeatureExplorer/GetFeature", {
    latitude: 410248224,
    longitude: -747127767
  });

  check(response, { "status is OK": (r) => r && r.status === grpc.StatusOK });
  
  if (!response.message) {
    throw new Error("Expected response message but got none");
  }
  
  client.close();
};
`, grpcServer.Addr)

	// Set up test state using the existing framework
	ts := getSingleFileTestState(t, script, []string{"-v", "--log-output=stdout", "--no-usage-report"}, 0)

	// Read the actual proto file from testutils (same as working gRPC test)
	protoContent, err := os.ReadFile("../../../testutils/grpcservice/route_guide.proto") //nolint:forbidigo
	require.NoError(t, err)

	// Provide the proto file as would be available in a separate process scenario
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "route_guide.proto"), protoContent, 0o644))

	// Execute the test
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	// Verify successful execution
	stdout := ts.Stdout.String()
	assert.Contains(t, stdout, "1 complete and 0 interrupted iterations")
	assert.Empty(t, ts.Stderr.String())
}
