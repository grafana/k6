package disruptors

import (
	"strings"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/testutils/command"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
	"github.com/grafana/xk6-disruptor/pkg/types/intstr"

	corev1 "k8s.io/api/core/v1"
)

func buildPodWithPort(name string, portName string, port int32) corev1.Pod {
	container := builders.NewContainerBuilder(name).
		WithPort(portName, port).
		Build()

	pod := builders.NewPodBuilder(name).
		WithNamespace("test-ns").
		WithContainer(container).
		WithIP("192.0.2.6").
		Build()

	return pod
}

func Test_PodHTTPFaultCommandGenerator(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		target      corev1.Pod
		expectedCmd string
		expectError bool
		cmdError    error
		fault       HTTPFault
		opts        HTTPDisruptionOptions
		duration    time.Duration
	}{
		{
			title:  "Test error 500",
			target: buildPodWithPort("my-app-pod", "http", 80),
			fault: HTTPFault{
				ErrorRate: 0.1,
				ErrorCode: 500,
				Port:      intstr.FromInt32(80),
			},
			opts:        HTTPDisruptionOptions{},
			duration:    60 * time.Second,
			expectedCmd: "xk6-disruptor-agent http -d 60s -t 80 -r 0.1 -e 500 --upstream-host 192.0.2.6",
			expectError: false,
			cmdError:    nil,
		},
		{
			title:  "Test error 500 with error body",
			target: buildPodWithPort("my-app-pod", "http", 80),
			// TODO: Make expectedCmd better represent the actual result ([]string), as it currently looks like we
			// are asserting a broken behavior (e.g. lack of quotes in -b) which is not the case.
			expectedCmd: "xk6-disruptor-agent http -d 60s -t 80 -r 0.1 -e 500 -b {\"error\": 500} --upstream-host 192.0.2.6",
			expectError: false,
			cmdError:    nil,
			fault: HTTPFault{
				ErrorRate: 0.1,
				ErrorCode: 500,
				ErrorBody: "{\"error\": 500}",
				Port:      intstr.FromInt32(80),
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60 * time.Second,
		},
		{
			title:       "Test Average delay",
			target:      buildPodWithPort("my-app-pod", "http", 80),
			expectedCmd: "xk6-disruptor-agent http -d 60s -t 80 -a 100ms -v 0ms --upstream-host 192.0.2.6",
			expectError: false,
			cmdError:    nil,
			fault: HTTPFault{
				AverageDelay: 100 * time.Millisecond,
				Port:         intstr.FromInt32(80),
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60 * time.Second,
		},
		{
			title:       "Test exclude list",
			target:      buildPodWithPort("my-app-pod", "http", 80),
			expectedCmd: "xk6-disruptor-agent http -d 60s -t 80 -x /path1,/path2 --upstream-host 192.0.2.6",
			expectError: false,
			cmdError:    nil,
			fault: HTTPFault{
				Exclude: "/path1,/path2",
				Port:    intstr.FromInt32(80),
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60 * time.Second,
		},
		{
			title:       "Container port not found",
			target:      buildPodWithPort("my-app-pod", "http", 80),
			expectedCmd: "",
			expectError: true,
			fault: HTTPFault{
				Port: intstr.FromInt32(8080),
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60,
		},
		{
			title: "Pod without PodIP",
			target: builders.NewPodBuilder("noip").
				WithNamespace("test-ns").
				WithLabel("app", "myapp").
				WithContainer(
					builders.NewContainerBuilder("noip").
						WithPort("http", 80).
						Build(),
				).
				Build(),
			expectedCmd: "",
			expectError: true,
			fault: HTTPFault{
				Port: intstr.FromInt32(80),
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60,
		},
		{
			title: "Pod with hostNetwork",
			target: builders.NewPodBuilder("hostnet").
				WithNamespace("test-ns").
				WithLabel("app", "myapp").
				WithHostNetwork(true).
				WithIP("192.0.2.6").
				WithContainer(
					builders.NewContainerBuilder("myapp").
						WithPort("http", 80).
						Build(),
				).
				Build(),
			expectedCmd: "",
			expectError: true,
			fault: HTTPFault{
				Port: intstr.FromInt32(80),
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			cmd := PodHTTPFaultCommand{
				fault:    tc.fault,
				duration: tc.duration,
				options:  tc.opts,
			}

			cmds, err := cmd.Commands(tc.target)
			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error : %v", err)
				return
			}

			if !command.AssertCmdEquals(strings.Join(cmds.Exec, " "), tc.expectedCmd) {
				t.Errorf("expected command: %s got: %s", tc.expectedCmd, cmds.Exec)
			}
		})
	}
}

func Test_PodGrpcPFaultCommandGenerator(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		target      corev1.Pod
		fault       GrpcFault
		opts        GrpcDisruptionOptions
		duration    time.Duration
		expectedCmd string
		expectError bool
		cmdError    error
	}{
		{
			title:  "Test error",
			target: buildPodWithPort("my-app-pod", "grpc", 3000),
			fault: GrpcFault{
				ErrorRate:  0.1,
				StatusCode: 14,
				Port:       intstr.FromInt32(3000),
			},
			opts:        GrpcDisruptionOptions{},
			duration:    60 * time.Second,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -t 3000 -r 0.1 -s 14 --upstream-host 192.0.2.6",
			expectError: false,
			cmdError:    nil,
		},
		{
			title:  "Test error with status message",
			target: buildPodWithPort("my-app-pod", "grpc", 3000),
			fault: GrpcFault{
				ErrorRate:     0.1,
				StatusCode:    14,
				StatusMessage: "internal error",
				Port:          intstr.FromInt32(3000),
			},
			opts:        GrpcDisruptionOptions{},
			duration:    60 * time.Second,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -t 3000 -r 0.1 -s 14 -m internal error --upstream-host 192.0.2.6",
			expectError: false,
			cmdError:    nil,
		},
		{
			title:  "Test Average delay",
			target: buildPodWithPort("my-app-pod", "grpc", 3000),
			fault: GrpcFault{
				AverageDelay: 100 * time.Millisecond,
				Port:         intstr.FromInt32(3000),
			},
			opts:        GrpcDisruptionOptions{},
			duration:    60 * time.Second,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -t 3000 -a 100ms -v 0ms --upstream-host 192.0.2.6",
			expectError: false,
			cmdError:    nil,
		},
		{
			title:  "Test exclude list",
			target: buildPodWithPort("my-app-pod", "grpc", 3000),
			fault: GrpcFault{
				Exclude: "service1,service2",
				Port:    intstr.FromInt32(3000),
			},
			opts:        GrpcDisruptionOptions{},
			duration:    60 * time.Second,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -t 3000 -x service1,service2 --upstream-host 192.0.2.6",
			expectError: false,
			cmdError:    nil,
		},
		{
			title:       "Container port not found",
			target:      buildPodWithPort("my-app-pod", "grpc", 3000),
			expectError: true,
			fault:       GrpcFault{Port: intstr.FromInt32(8080)},
			opts:        GrpcDisruptionOptions{},
			duration:    60,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			cmd := PodGrpcFaultCommand{
				fault:    tc.fault,
				duration: tc.duration,
				options:  tc.opts,
			}

			cmds, err := cmd.Commands(tc.target)

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error : %v", err)
				return
			}

			if !command.AssertCmdEquals(strings.Join(cmds.Exec, " "), tc.expectedCmd) {
				t.Errorf("expected command: %s got: %s", tc.expectedCmd, cmds.Exec)
			}
		})
	}
}
