package tests

import (
	"os"
	"path/filepath"
	"testing"

	"go.k6.io/k6/internal/cmd"
	"go.k6.io/k6/lib/fsext"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const projectRootPath = "../../../"

// TestGRPCInputOutput runs same k6's scripts that we have in example folder
// it check that output contains/not contains cetane things
func TestGRPCInputOutput(t *testing.T) {
	t.Parallel()

	tc := map[string]struct {
		script                 string
		outputShouldContain    []string
		outputShouldNotContain []string
	}{
		"Server streaming": {
			script: projectRootPath + "examples/grpc_server_streaming.js",
			outputShouldContain: []string{
				"output: -",
				"default: 1 iterations for each of 1 VUs",
				"1 complete and 0 interrupted iterations",
				"Found feature called",
				"grpc_streams",
				"grpc_streams_msgs_received",
				"grpc_streams_msgs_sent",
				"All done",
			},
			outputShouldNotContain: []string{
				"Stream Error:",
			},
		},
		"Client Streaming": {
			script: projectRootPath + "examples/grpc_client_streaming.js",
			outputShouldContain: []string{
				"output: -",
				"default: 1 iterations for each of 1 VUs",
				"1 complete and 0 interrupted iterations",
				"Visiting point",
				"Finished trip with 5 points",
				"Passed 5 feature",
				"grpc_streams",
				"grpc_streams_msgs_received",
				"grpc_streams_msgs_sent",
				"All done",
			},
			outputShouldNotContain: []string{
				"Stream Error:",
			},
		},
		"Invoke": {
			script: projectRootPath + "examples/grpc_invoke.js",
			outputShouldContain: []string{
				"output: -",
				"default: 1 iterations for each of 1 VUs",
				"1 complete and 0 interrupted iterations",
				"3 Hasta Way, Newton, NJ 07860, USA",
			},
			outputShouldNotContain: []string{
				"grpc_streams",
				"grpc_streams_msgs_received",
				"grpc_streams_msgs_sent",
			},
		},
		"Reflection": {
			script: projectRootPath + "examples/grpc_reflection.js",
			outputShouldContain: []string{
				"output: -",
				"default: 1 iterations for each of 1 VUs",
				"1 complete and 0 interrupted iterations",
				"3 Hasta Way, Newton, NJ 07860, USA",
			},
			outputShouldNotContain: []string{
				"grpc_streams",
				"grpc_streams_msgs_received",
				"grpc_streams_msgs_sent",
			},
		},
	}

	// Read the proto file from the testutils package
	// it's same that we use in the examples
	proto, err := os.ReadFile(projectRootPath + "internal/lib/testutils/grpcservice/route_guide.proto") //nolint:forbidigo
	require.NoError(t, err)

	for name, test := range tc {
		name := name
		test := test

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tb := NewGRPC(t)

			script, err := os.ReadFile(test.script) //nolint:forbidigo
			require.NoError(t, err)

			ts := getSingleFileTestState(t, string(script), []string{"-v", "--log-output=stdout", "--no-usage-report"}, 0)
			ts.Env = map[string]string{
				"GRPC_ADDR":       tb.Addr,
				"GRPC_PROTO_PATH": "./proto.proto",
			}
			require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "proto.proto"), proto, 0o644))

			cmd.ExecuteWithGlobalState(ts.GlobalState)

			stdout := ts.Stdout.String()

			for _, s := range test.outputShouldContain {
				assert.Contains(t, stdout, s)
			}
			for _, s := range test.outputShouldNotContain {
				assert.NotContains(t, stdout, s)
			}

			assert.Empty(t, ts.Stderr.String())
		})
	}
}
