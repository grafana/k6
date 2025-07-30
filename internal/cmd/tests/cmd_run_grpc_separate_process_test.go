package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)



const (
	serverScript = `package main

import (
	"fmt"
	"log"
	"net"

	"go.k6.io/k6/testutils/grpcservice"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	addr := "%s"
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %%v", err)
	}

	grpcServer := grpc.NewServer()
	features := grpcservice.LoadFeatures("")
	server := grpcservice.NewRouteGuideServer(features...)
	grpcservice.RegisterRouteGuideServer(grpcServer, server)
	reflection.Register(grpcServer)

	// Print available services
	services := grpcServer.GetServiceInfo()
	fmt.Printf("Available services:\n")
	for svc, info := range services {
		fmt.Printf("Service: %%s\n", svc)
		fmt.Printf("  Methods:\n")
		for _, method := range info.Methods {
			fmt.Printf("    - %%s\n", method.Name)
		}
	}

	fmt.Printf("Server started at %%s\n", addr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %%v", err)
	}
}`

	k6Script = `
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const client = new grpc.Client();
client.load([], 'proto/route_guide.proto');

export default function() {
    client.connect("%s", {
        timeout: '5s',
        plaintext: true
    });

    console.log('Connected to server, attempting to call GetFeature...');

    try {
        const response = client.invoke('routeguide.RouteGuide/GetFeature', {
            latitude: 409146138,
            longitude: -746188906
        });

        console.log('Response status:', response.status);
        console.log('Response error:', response.error);
        console.log('Response message:', JSON.stringify(response.message));

        check(response, {
            'status is OK': (r) => r.status === grpc.StatusOK,
            'has no error': (r) => !r.error,
            'has message': (r) => r.message !== null,
        });
    } catch (err) {
        console.error('Error:', err);
        console.error('Error details:', JSON.stringify(err, null, 2));
        throw err;
    } finally {
        client.close();
    }
}`

	protoFile = `syntax = "proto3";

package routeguide;

// Interface exported by the server.
service RouteGuide {
  // A simple RPC.
  //
  // Obtains the feature at a given position.
  rpc GetFeature(Point) returns (Feature) {}
}

// A point on the Earth.
message Point {
  int32 latitude = 1;
  int32 longitude = 2;
}

// A feature names something at a given point.
message Feature {
  // The name of the feature.
  string name = 1;

  // The point where the feature is detected.
  Point location = 2;
}`
)

// TestGRPCSeparateProcess tests that the gRPC client can properly load proto definitions
// when running in a separate process from the server.
func TestGRPCSeparateProcess(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for our test files
	tmpDir, err := os.MkdirTemp("", "grpc-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Get a free port for the gRPC server
	addr := getFreeBindAddr(t)

	// Create the server program
	serverFile := filepath.Join(tmpDir, "server.go")
	err = os.WriteFile(serverFile, []byte(fmt.Sprintf(serverScript, addr)), 0644)
	require.NoError(t, err)

	// Build the server
	serverBin := filepath.Join(tmpDir, "server")
	cmd := exec.Command("go", "build", "-o", serverBin, serverFile)
	cmd.Env = append(os.Environ(), "GO111MODULE=auto")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build server: %s", output)

	// Start the server
	serverCmd := exec.Command(serverBin)
	serverCmd.Stdout = os.Stdout
	serverCmd.Stderr = os.Stderr
	require.NoError(t, serverCmd.Start())
	defer serverCmd.Process.Kill()

	// Wait for the server to start
	time.Sleep(2 * time.Second)

	// Create the k6 test script
	scriptFile := filepath.Join(tmpDir, "script.js")
	err = os.WriteFile(scriptFile, []byte(fmt.Sprintf(k6Script, addr)), 0644)
	require.NoError(t, err)

	// Create a directory for the proto file
	protoDir := filepath.Join(tmpDir, "proto")
	err = os.MkdirAll(protoDir, 0755)
	require.NoError(t, err)

	// Write the proto file
	protoDst := filepath.Join(protoDir, "route_guide.proto")
	err = os.WriteFile(protoDst, []byte(protoFile), 0644)
	require.NoError(t, err)

	// Print proto file location for debugging
	fmt.Printf("Proto file location: %s\n", protoDst)
	fmt.Printf("Proto file contents:\n%s\n", protoFile)

	// Get project root for k6 binary - walk up to find go.mod
	projectRoot := func() string {
		dir, err := os.Getwd()
		require.NoError(t, err)
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				t.Fatal("Could not find project root")
			}
			dir = parent
		}
	}()

	// Build k6
	k6Bin := filepath.Join(tmpDir, "k6")
	cmd = exec.Command("go", "build", "-o", k6Bin, ".")
	cmd.Dir = projectRoot
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build k6: %s", output)

	// Run k6 with the test script
	k6Cmd := exec.Command(k6Bin, "run", "--quiet", scriptFile)
	k6Cmd.Dir = tmpDir // Change working directory to where the proto file is
	k6Cmd.Env = append(os.Environ(),
		"GO111MODULE=auto",
		"K6_LOG_LEVEL=debug",
	)
	k6Cmd.Stdout = os.Stdout
	k6Cmd.Stderr = os.Stderr
	err = k6Cmd.Run()
	require.NoError(t, err, "k6 test failed")
}
