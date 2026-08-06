//go:build grpc_e2e

// Package grpce2e runs k6 against a real gRPC server, each in its own OS
// process. See doc.go and https://github.com/grafana/k6/issues/3552.
package grpce2e

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// protoPath is the location of the RouteGuide .proto file, expressed relative
// to the directory of the example scripts (examples/), because k6 resolves
// load() paths relative to the script's directory.
const protoPath = "../internal/lib/testutils/grpcservice/route_guide.proto"

// repoRoot returns the absolute path of the repository root, derived from the
// location of this test file.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "could not determine caller location")
	// this file lives at <root>/internal/e2e/grpc/grpc_e2e_test.go
	root, err := filepath.Abs(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
	require.NoError(t, err)
	return root
}

// goBuild compiles the package at pkg into an executable at out, using the same
// toolchain that runs the tests.
func goBuild(t *testing.T, root, out, pkg string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "-o", out, pkg)
	cmd.Dir = root
	if buildOut, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build %s: %v\n%s", pkg, err, buildOut)
	}
}

// startServer starts the helper gRPC server (already built at serverBin) in its
// own process and returns the address it is listening on. The server is stopped
// when the test's context is cancelled (i.e. during cleanup).
func startServer(t *testing.T, serverBin string) string {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), serverBin, "-addr", "127.0.0.1:0")
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.Stderr = os.Stderr //nolint:forbidigo // surfacing server logs on failure is helpful
	require.NoError(t, cmd.Start(), "failed to start gRPC server")

	t.Cleanup(func() { _ = cmd.Wait() })

	// The server prints "LISTENING <addr>" once it is bound and ready.
	addrCh := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if rest, ok := strings.CutPrefix(scanner.Text(), "LISTENING "); ok {
				addrCh <- strings.TrimSpace(rest)
				return
			}
		}
	}()

	select {
	case addr := <-addrCh:
		return addr
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for gRPC server to start listening")
		return ""
	}
}

// runK6 runs `k6 run <script>` as a separate process against the given gRPC
// address and returns the combined output and the run error (nil on exit 0).
func runK6(t *testing.T, k6Bin, root, script, addr, proto string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, k6Bin,
		"run", "--no-usage-report", "--quiet",
		filepath.Join("examples", script),
	)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), //nolint:forbidigo // inherit the toolchain environment (PATH, HOME, ...)
		"GRPC_ADDR="+addr,
		"GRPC_PROTO_PATH="+proto,
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// TestGRPCSeparateProcess verifies that k6 can load gRPC proto definitions at
// runtime and talk to a real server while running in a separate OS process from
// that server. Because the k6 process never links the server's generated proto
// types, it cannot piggyback on the process-global protobuf registry; it must
// load the types itself from the .proto file or from server reflection.
func TestGRPCSeparateProcess(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain not available: %v", err)
	}

	root := repoRoot(t)
	binDir := t.TempDir()

	k6Bin := filepath.Join(binDir, "k6")
	serverBin := filepath.Join(binDir, "grpcserver")
	goBuild(t, root, k6Bin, ".")
	goBuild(t, root, serverBin, "./internal/e2e/grpc/testserver")

	addr := startServer(t, serverBin)
	t.Logf("gRPC server listening on %s", addr)

	// Positive cases: each exercises a different code path, all loading types at
	// runtime rather than from a shared in-process registry.
	cases := []struct {
		name        string
		script      string
		wantContain []string
	}{
		{
			name:        "unary_invoke_load_from_proto",
			script:      "grpc_invoke.js",
			wantContain: []string{"3 Hasta Way, Newton, NJ 07860, USA"},
		},
		{
			name:        "server_streaming_load_from_proto",
			script:      "grpc_server_streaming.js",
			wantContain: []string{"Found feature called", "All done"},
		},
		{
			name:        "client_streaming_load_from_proto",
			script:      "grpc_client_streaming.js",
			wantContain: []string{"Finished trip with", "All done"},
		},
		{
			name:        "unary_invoke_via_reflection",
			script:      "grpc_reflection.js",
			wantContain: []string{"3 Hasta Way, Newton, NJ 07860, USA"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, err := runK6(t, k6Bin, root, tc.script, addr, protoPath)
			require.NoErrorf(t, err, "k6 run %s failed:\n%s", tc.script, out)
			for _, want := range tc.wantContain {
				assert.Containsf(t, out, want, "output of %s:\n%s", tc.script, out)
			}
		})
	}

	// Negative control: if proto loading is broken (here: the .proto file cannot
	// be found/parsed), the run MUST fail. This guards against a regression that
	// silently succeeds without actually loading the definitions.
	t.Run("missing_proto_fails", func(t *testing.T) {
		t.Parallel()
		out, err := runK6(t, k6Bin, root, "grpc_invoke.js", addr, "./does-not-exist.proto")
		require.Errorf(t, err, "k6 run must fail when the .proto cannot be loaded, got:\n%s", out)
	})
}
