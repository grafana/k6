package tcpconn_test

import (
	"fmt"
	"io"
	"log"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const iperfPort = "6666"

// Benchmark_DisruptorThroughput benchmarks the impact of deploying the agent with no disruptions set up.
// It measures the time it takes to run an iperf3 test that sends a fixed amount of data.
func Benchmark_DisruptorThroughput(b *testing.B) {
	nilLogger := log.New(io.Discard, "", 0)

	// Disable built-in metric, as we do not care about it.
	b.ReportMetric(0, "ns/op")

	for _, bc := range []struct {
		name          string
		disruptionCmd []string
	}{
		{
			name:          "agent-disabled",
			disruptionCmd: []string{"sleep", "infinity"},
		},
		{
			name: "accept-all",
			disruptionCmd: []string{
				"xk6-disruptor-agent", "tcp-drop", "-d", "1h", "--port", iperfPort, "--rate", "0.0",
			},
		},
	} {
		b.Run("disruption="+bc.name, func(b *testing.B) {
			ctx := b.Context()

			iperfServer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				Logger:       nilLogger,
				ProviderType: testcontainers.ProviderDocker,
				ContainerRequest: testcontainers.ContainerRequest{
					FromDockerfile: testcontainers.FromDockerfile{
						Dockerfile: "Dockerfile",
						Context:    filepath.Join("..", "..", "..", "testcontainers", "iperf3"),
					},
					ExposedPorts: []string{iperfPort},
					Entrypoint:   []string{"iperf3", "-sp", iperfPort},
					WaitingFor: wait.ForAll(
						wait.ForExposedPort(),
					),
				},
				Started: true,
			})
			if err != nil {
				b.Fatalf("creating iperf3 server container: %v", err)
			}
			b.Cleanup(func() {
				_ = iperfServer.Terminate(ctx)
			})

			agentSidecar, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				Logger:       nilLogger,
				ProviderType: testcontainers.ProviderDocker,
				ContainerRequest: testcontainers.ContainerRequest{
					Image:       "ghcr.io/grafana/xk6-disruptor-agent",
					Entrypoint:  bc.disruptionCmd,
					Privileged:  true,
					NetworkMode: container.NetworkMode("container:" + iperfServer.GetContainerID()),
				},
				Started: true,
			})
			if err != nil {
				b.Fatalf("creating agent container: %v", err)
			}
			b.Cleanup(func() {
				_ = agentSidecar.Terminate(ctx)
			})

			port, err := iperfServer.MappedPort(ctx, nat.Port(iperfPort))
			if err != nil {
				b.Fatalf("getting iperf3 mapped port: %v", err)
			}

			iperfClient, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				Logger:       nilLogger,
				ProviderType: testcontainers.ProviderDocker,
				ContainerRequest: testcontainers.ContainerRequest{
					FromDockerfile: testcontainers.FromDockerfile{
						Dockerfile: "Dockerfile",
						Context:    filepath.Join("..", "..", "..", "testcontainers", "iperf3"),
					},
					// Start container paused, so the benchmark captures only the command execution later and not the
					// container startup time.
					Entrypoint: []string{"sleep", "infinity"},
					// To avoid connecting to iperf3 through localhost, we run the client with net=host and connect
					// to the mapped port.
					NetworkMode: "host",
				},
				Started: true,
			})
			if err != nil {
				b.Fatalf("creating agent container: %v", err)
			}

			b.Cleanup(func() {
				_ = iperfClient.Terminate(ctx)
			})

			// TODO: Find a better way to wait for disruption to start.
			time.Sleep(3 * time.Second)

			// Use iperf3 to send a fixed amount of data. We will measure how long the command takes to complete.
			const megabytes = 1024
			cmd := []string{"iperf3", "-c", "127.0.0.1", "-p", port.Port(), "-n", fmt.Sprintf("%dM", megabytes)}

			// TODO: b.Elapsed is a better way to do this, but it is not available in the Go version we are using.
			start := time.Now()
			// We run the iperf3 command as many times as b.N tells us to.
			for range b.N {
				rc, outputReader, err := iperfClient.Exec(ctx, cmd)
				if err != nil {
					b.Fatalf("error running iperf client command: %v", err)
				}
				if rc != 0 {
					output, _ := io.ReadAll(outputReader)
					b.Fatalf("iperf3 returned %d:\n%s", rc, string(output))
				}
			}

			b.ReportMetric(megabytes*float64(b.N)/time.Since(start).Seconds(), "MiB/s")
		})
	}
}
