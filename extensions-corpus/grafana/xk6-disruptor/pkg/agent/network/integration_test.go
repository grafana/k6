//go:build integration
// +build integration

package network_test

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/docker/docker/api/types/container"
	"github.com/grafana/xk6-disruptor/pkg/testutils/echotester"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const echoServerPort = "6666"

type DisruptionConfig struct {
	Duration time.Duration
	Port     uint
	Protocol string
}

func CreateEchoServer(t *testing.T) (testcontainers.Container, nat.Port) {
	ctx := context.Background()

	echoserver, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ProviderType: testcontainers.ProviderDocker,
		ContainerRequest: testcontainers.ContainerRequest{
			ExposedPorts: []string{echoServerPort},
			FromDockerfile: testcontainers.FromDockerfile{
				Dockerfile: "Dockerfile",
				Context:    filepath.Join("..", "..", "..", "testcontainers", "echoserver"),
			},
			WaitingFor: wait.ForExposedPort(),
		},
		Started: true,
	})
	require.NoError(t, err, "creating echoserver container")

	port, err := echoserver.MappedPort(ctx, nat.Port(echoServerPort))
	require.NoError(t, err, "getting echoserver mapped port")

	t.Cleanup(func() { require.NoError(t, echoserver.Terminate(ctx)) })

	return echoserver, port
}

func CreateAgentWithDisruptionConfig(t *testing.T, target testcontainers.Container, config DisruptionConfig) {
	ctx := context.Background()

	args := []string{"xk6-disruptor-agent", "network-drop", "-d", fmt.Sprint(config.Duration)}

	if config.Port > 0 {
		args = append(args, "-p", fmt.Sprint(config.Port))
	}

	if config.Protocol != "" {
		args = append(args, "-P", config.Protocol)
	}

	agentSidecar, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ProviderType: testcontainers.ProviderDocker,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:       "ghcr.io/grafana/xk6-disruptor-agent:latest",
			Entrypoint:  args,
			Privileged:  true,
			WaitingFor:  wait.ForExec([]string{"pgrep", "xk6-disruptor-agent"}),
			NetworkMode: container.NetworkMode("container:" + target.GetContainerID()),
		},
		Started: true,
	})
	require.NoError(t, err, "creating agent container")
	t.Cleanup(func() { require.NoError(t, agentSidecar.Terminate(ctx)) })
}

func runDisruptionTest(t *testing.T, config DisruptionConfig, expectedFailures int, waitTime time.Duration) {
	// Create echo server
	echoserver, port := CreateEchoServer(t)

	// Create agent with disruption config
	CreateAgentWithDisruptionConfig(t, echoserver, config)

	// Wait specified time
	time.Sleep(waitTime)

	// Run test connections
	errors := make(chan error)
	const nTests = 10

	for range nTests {
		go func() {
			time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
			echoTester, err := echotester.NewTester(net.JoinHostPort("localhost", port.Port()))
			if err != nil {
				errors <- err
				return
			}
			errors <- echoTester.Echo(5 * time.Second)
		}()
	}

	nErrors := 0
	for range nTests {
		if err := <-errors; err != nil {
			nErrors++
		}
	}

	// Check expected results
	if nErrors != expectedFailures {
		t.Errorf("got %d errors, expected %d", nErrors, expectedFailures)
	}

	t.Logf("Got %d errors out of %d tests (expected %d)", nErrors, nTests, expectedFailures)
}

// Test_NetworkDrop_Scenarios tests different network disruption scenarios
func Test_NetworkDrop_Scenarios(t *testing.T) {
	t.Parallel()

	t.Run("Default", func(t *testing.T) {
		t.Parallel()
		// Default disruption should drop all traffic
		runDisruptionTest(t, DisruptionConfig{
			Duration: time.Hour,
			// No port or protocol specified - should drop all INPUT traffic
		}, 10, 1*time.Second) // Expect all 10 to fail
	})

	t.Run("SpecificPortAndProtocol", func(t *testing.T) {
		t.Parallel()
		// Targeting specific port and protocol should drop matching traffic
		runDisruptionTest(t, DisruptionConfig{
			Duration: time.Hour,
			Port:     6666, // This should match echoServerPort
			Protocol: "tcp",
		}, 10, 1*time.Second) // Expect all 10 to fail
	})

	t.Run("DifferentPortAndProtocol", func(t *testing.T) {
		t.Parallel()
		// Targeting different port should not affect echo server
		runDisruptionTest(t, DisruptionConfig{
			Duration: time.Hour,
			Port:     9999, // Different port from echoServerPort
			Protocol: "tcp",
		}, 0, 1*time.Second) // Expect 0 to fail
	})

	t.Run("StopsAfterDuration", func(t *testing.T) {
		t.Parallel()
		// Short duration disruption should stop, allowing connections to succeed
		runDisruptionTest(t, DisruptionConfig{
			Duration: 3 * time.Second,
			// Default disruption - drops all INPUT traffic
		}, 0, 5*time.Second) // Wait 5 seconds for disruption to end, expect 0 to fail
	})
}
