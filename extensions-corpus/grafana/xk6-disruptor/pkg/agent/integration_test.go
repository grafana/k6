//go:build integration
// +build integration

package agent

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/grafana/xk6-disruptor/pkg/testutils/grpc/dynamic"
	tcutils "github.com/grafana/xk6-disruptor/pkg/testutils/testcontainers"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func injectHTTPError(rate float64, code int, upstream string) []string {
	return []string{
		"xk6-disruptor-agent",
		"http",
		"--duration",
		"300s",
		"--rate",
		fmt.Sprintf("%.2f", rate),
		"--error",
		fmt.Sprintf("%d", code),
		"--port",
		"8080",
		"--target",
		"80",
		"--upstream-host",
		upstream,
	}
}

func injectGrpcFault(rate float64, status int, upstream string) []string {
	return []string{
		"xk6-disruptor-agent",
		"grpc",
		"--duration",
		"300s",
		"--rate",
		fmt.Sprintf("%.2f", rate),
		"--status",
		fmt.Sprintf("%d", status),
		"--message",
		"Internal error",
		"--port",
		"4000",
		"--target",
		"9000",
		"-x",
		// exclude reflection service otherwise the dynamic client will not work
		"grpc.reflection.v1alpha.ServerReflection,grpc.reflection.v1.ServerReflection",
		"--upstream-host",
		upstream,
	}
}

func injectHTTPError418NonTransparent(upstream string) []string {
	return []string{
		"xk6-disruptor-agent",
		"http",
		"--transparent=false", // must have an '=', otherwise has no effect
		"--duration",
		"300s",
		"--rate",
		"1.0",
		"--error",
		"418",
		"--port",
		"8080",
		"--target",
		"80",
		"--upstream-host",
		upstream,
	}
}

func checkHTTPRequest(url string, expected int) error {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request %w", err)
	}

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed request to %q: %v", url, err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != expected {
		return fmt.Errorf("expected status code %d but %d received", expected, resp.StatusCode)
	}

	return nil
}

func checkGrpcRequest(host string, service string, method string, request []byte, expected int32) error {
	client, err := dynamic.NewClientWithDialOptions(
		host,
		service,
		grpc.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("creating client for service %s: %w", service, err)
	}

	err = client.Connect(context.TODO())
	if err != nil {
		return fmt.Errorf("connecting to service %s: %w", service, err)
	}

	input := [][]byte{}
	input = append(input, request)

	_, err = client.Invoke(context.TODO(), method, input)
	// got an error but it is not due to the grpc status
	s, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("unexpected error: %w", err)
	}

	if int32(s.Code()) != expected {
		return fmt.Errorf("expected status code %d got %d", expected, int32(s.Code()))
	}

	return nil
}

func Test_HTTPFaultInjection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		test    string
		rate    float64
		code    int
		request int
		expect  int
	}{
		{
			test:    "inject 418 error",
			rate:    1.0,
			code:    418,
			request: 200,
			expect:  418,
		},
		{
			test:    "inject no error",
			rate:    0.0,
			code:    0,
			request: 200,
			expect:  200,
		},
		{
			test:    "handle upstream error",
			rate:    0.0,
			code:    0,
			request: 500,
			expect:  500,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			gcr := testcontainers.GenericContainerRequest{
				ProviderType: testcontainers.ProviderDocker,
				ContainerRequest: testcontainers.ContainerRequest{
					Image: "kennethreitz/httpbin",
					ExposedPorts: []string{
						"80",
					},
					WaitingFor: wait.ForExposedPort(),
				},
				Started: true,
			}
			httpbin, err := testcontainers.GenericContainer(ctx, gcr)
			if err != nil {
				t.Fatalf("failed to create httpbin container %v", err)
			}

			t.Cleanup(func() {
				_ = httpbin.Terminate(ctx)
			})

			httpbinIP, err := httpbin.ContainerIP(ctx)
			if err != nil {
				t.Fatalf("failed to get httpbin IP:\n%v", err)
			}

			// make the agent run using the same stack than the httpbin container
			httpbinNetwork := container.NetworkMode("container:" + httpbin.GetContainerID())
			gcr = testcontainers.GenericContainerRequest{
				ProviderType: testcontainers.ProviderDocker,
				ContainerRequest: testcontainers.ContainerRequest{
					Image:       "ghcr.io/grafana/xk6-disruptor-agent",
					NetworkMode: httpbinNetwork,
					Cmd:         injectHTTPError(tc.rate, tc.code, httpbinIP),
					Privileged:  true,
					WaitingFor:  wait.ForExec([]string{"pgrep", "xk6-disruptor-agent"}),
				},
				Started: true,
			}

			agent, err := testcontainers.GenericContainer(ctx, gcr)
			if err != nil {
				t.Fatalf("failed to create agent container %v", err)
			}

			t.Cleanup(func() {
				_ = agent.Terminate(ctx)
			})

			httpPort, err := httpbin.MappedPort(ctx, "80")
			if err != nil {
				t.Fatalf("failed to get httpbin port:\n%v", err)
			}

			// access httpbin
			url := fmt.Sprintf("http://localhost:%s/status/%d", httpPort.Port(), tc.request)

			err = checkHTTPRequest(url, tc.expect)
			if err != nil {
				t.Fatalf("failed %v", err)
			}
		})
	}
}

func Test_GrpcFaultInjection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		test   string
		rate   float64
		status int
		expect int32
	}{
		{
			test:   "inject internal error",
			rate:   1.0,
			status: 14,
			expect: 14,
		},
		{
			test:   "inject no error",
			rate:   0.0,
			status: 14,
			expect: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			exposed, _ := nat.NewPort("tcp", "9000")
			gcr := testcontainers.GenericContainerRequest{
				ProviderType: testcontainers.ProviderDocker,
				ContainerRequest: testcontainers.ContainerRequest{
					Image: "moul/grpcbin",
					ExposedPorts: []string{
						"9000",
					},
					WaitingFor: wait.ForListeningPort(exposed),
				},
				Started: true,
			}
			grpcbin, err := testcontainers.GenericContainer(ctx, gcr)
			if err != nil {
				t.Fatalf("failed to create grpcbin container: %v", err)
			}

			t.Cleanup(func() {
				_ = grpcbin.Terminate(ctx)
			})

			grpcbinIP, err := grpcbin.ContainerIP(ctx)
			if err != nil {
				t.Fatalf("failed to get grpcbin IP:\n%v", err)
			}

			// make the agent run using the same stack than the grpcbin container
			grpcbinNetwork := container.NetworkMode("container:" + grpcbin.GetContainerID())
			gcr = testcontainers.GenericContainerRequest{
				ProviderType: testcontainers.ProviderDocker,
				ContainerRequest: testcontainers.ContainerRequest{
					Image:       "ghcr.io/grafana/xk6-disruptor-agent",
					NetworkMode: grpcbinNetwork,
					Cmd:         injectGrpcFault(tc.rate, tc.status, grpcbinIP),
					Privileged:  true,
					WaitingFor:  wait.ForExec([]string{"pgrep", "xk6-disruptor-agent"}),
				},
				Started: true,
			}

			agent, err := testcontainers.GenericContainer(ctx, gcr)
			if err != nil {
				t.Fatalf("failed to create agent container %v", err)
			}

			t.Cleanup(func() {
				_ = agent.Terminate(ctx)
			})

			grpcPort, err := grpcbin.MappedPort(ctx, "9000")
			if err != nil {
				t.Fatalf("failed to get httpbin port:\n%v", err)
			}

			// access grpcbin
			url := fmt.Sprintf("localhost:%s", grpcPort.Port())

			err = checkGrpcRequest(url, "grpcbin.GRPCBin", "Empty", []byte("{}"), tc.expect)
			if err != nil {
				t.Fatalf("failed %v", err)
			}
		})
	}
}

func Test_Multiple_Executions(t *testing.T) {
	t.Parallel()

	ctx := context.TODO()

	// start the container with the agent using a fake upstream address because the agent requires it
	gcr := testcontainers.GenericContainerRequest{
		ProviderType: testcontainers.ProviderDocker,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:      "ghcr.io/grafana/xk6-disruptor-agent",
			Cmd:        injectHTTPError(1.0, 500, "1.1.1.1"),
			Privileged: true,
			WaitingFor: wait.ForExec([]string{"pgrep", "xk6-disruptor-agent"}),
		},
		Started: true,
	}

	agent, err := testcontainers.GenericContainer(ctx, gcr)
	if err != nil {
		t.Fatalf("failed to create agent container %v", err)
	}
	t.Cleanup(func() {
		_ = agent.Terminate(ctx)
	})

	// try to execute the agent again. Arguments are irrelevant
	_, output, err := agent.Exec(ctx, injectHTTPError(1.0, 500, "1.1.1.1"))
	if err != nil {
		t.Fatalf("command failed %v", err)
	}

	buffer := &bytes.Buffer{}
	buffer.ReadFrom(output)
	if !strings.Contains(buffer.String(), "is already running") {
		t.Fatalf("failed: %s: ", buffer.String())
	}
}

func Test_Non_Transparent_Proxy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	gcr := testcontainers.GenericContainerRequest{
		ProviderType: testcontainers.ProviderDocker,
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "kennethreitz/httpbin",
			ExposedPorts: []string{
				"80",
				"8080", // expose port the agent will use
			},
			WaitingFor: wait.ForExposedPort(),
		},
		Started: true,
	}
	httpbin, err := testcontainers.GenericContainer(ctx, gcr)
	if err != nil {
		t.Fatalf("failed to create httpbin container %v", err)
	}

	t.Cleanup(func() {
		_ = httpbin.Terminate(ctx)
	})

	httpbinIP, err := httpbin.ContainerIP(ctx)
	if err != nil {
		t.Fatalf("failed to get httpbin IP:\n%v", err)
	}

	// make the agent run using the same stack than the httpbin container
	httpbinNetwork := container.NetworkMode("container:" + httpbin.GetContainerID())
	gcr = testcontainers.GenericContainerRequest{
		ProviderType: testcontainers.ProviderDocker,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:       "ghcr.io/grafana/xk6-disruptor-agent",
			NetworkMode: httpbinNetwork,
			Cmd:         injectHTTPError418NonTransparent(httpbinIP),
			Privileged:  true,
			WaitingFor:  wait.ForExec([]string{"pgrep", "xk6-disruptor-agent"}),
		},
		Started: true,
	}

	agent, err := testcontainers.GenericContainer(ctx, gcr)
	if err != nil {
		t.Fatalf("failed to create agent container %v", err)
	}

	t.Cleanup(func() {
		_ = agent.Terminate(ctx)
	})

	httpPort, err := httpbin.MappedPort(ctx, "80")
	if err != nil {
		t.Fatalf("failed to get httpbin port:\n%v", err)
	}

	// access httpbin. Should get request's response
	httpUrl := fmt.Sprintf("http://localhost:%s/status/200", httpPort.Port())

	err = checkHTTPRequest(httpUrl, 200)
	if err != nil {
		t.Fatalf("failed %v", err)
	}

	// get agent's port. It is exposed in the httpBin container (both containers share the network stack)
	agentPort, err := httpbin.MappedPort(ctx, "8080")
	if err != nil {
		t.Fatalf("failed to get agent port:\n%v", err)
	}

	// access agent, should get injected error
	agentUrl := fmt.Sprintf("http://localhost:%s/status/200", agentPort.Port())

	err = checkHTTPRequest(agentUrl, 418)
	if err != nil {
		t.Fatalf("failed %v", err)
	}
}

func stressCPU(load uint) []string {
	return []string{
		"xk6-disruptor-agent",
		"stress",
		"--duration",
		"20s",
		"--load",
		fmt.Sprintf("%d", load),
	}
}

const cpuLoadTolerance = 10

func Test_CPUStressor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title string
		load  uint
	}{
		{
			title: "100% CPU",
			load:  100,
		},
		{
			title: "80% CPU",
			load:  80,
		},
		{
			title: "60% CPU",
			load:  60,
		},
		{
			title: "40% CPU",
			load:  40,
		},
		{
			title: "20% CPU",
			load:  20,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			ctx := context.TODO()

			gcr := testcontainers.GenericContainerRequest{
				ProviderType: testcontainers.ProviderDocker,
				ContainerRequest: testcontainers.ContainerRequest{
					Image:      "ghcr.io/grafana/xk6-disruptor-agent",
					Cmd:        stressCPU(tc.load),
					Privileged: false,
				},
				Started: true,
			}

			agent, err := testcontainers.GenericContainer(ctx, gcr)
			if err != nil {
				t.Fatalf("failed to create agent container %v", err)
			}
			t.Cleanup(func() {
				_ = agent.Terminate(ctx)
			})

			// collect stats while the container is running.
			// the Stats method has a 1 second delay, therefore it is not
			// necessary to sleep in the loop between iterations
			avgCPU := 0.0
			iterations := 0
			for {
				state, err := agent.State(ctx)
				if err != nil {
					t.Fatalf("getting container status %v", err)
				}

				if !state.Running {
					break
				}

				s, err := tcutils.Stats(context.TODO(), agent.GetContainerID())
				if err != nil {
					t.Fatalf("getting container stats %v", err)
				}

				avgCPU += s.CPUPercentage
				iterations++
			}

			if iterations == 0 {
				t.Fatalf("no stats for container")
			}

			avgCPU = avgCPU / float64(iterations)

			if math.Abs(avgCPU-float64(tc.load)) > float64(tc.load)*cpuLoadTolerance {
				t.Fatalf("Average CPU expected: %d got %f (tolerance %d%%)", tc.load, avgCPU, cpuLoadTolerance)
			}
		})
	}
}
