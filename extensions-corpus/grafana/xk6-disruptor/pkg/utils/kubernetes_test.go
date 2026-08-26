package utils

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"

	k8sintstr "k8s.io/apimachinery/pkg/util/intstr"

	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
	"github.com/grafana/xk6-disruptor/pkg/types/intstr"
)

func buildPodWithPort(name string, portName string, port int32) corev1.Pod {
	container := builders.NewContainerBuilder(name).
		WithPort(portName, port).
		Build()

	pod := builders.NewPodBuilder(name).
		WithContainer(container).
		Build()

	return pod
}

func buildServicWithPort(name string, portName string, port int32, target k8sintstr.IntOrString) corev1.Service {
	return builders.NewServiceBuilder(name).
		WithNamespace("test-ns").
		WithSelectorLabel("app", "test").
		WithPort(portName, port, target).
		Build()
}

func Test_FindPort(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		pod         corev1.Pod
		port        intstr.IntOrString
		expectError bool
		expected    intstr.IntOrString
	}{
		{
			title:       "Numeric port",
			pod:         buildPodWithPort("pod-1", "http", 80),
			port:        intstr.FromInt32(80),
			expectError: false,
			expected:    intstr.FromInt32(80),
		},
		{
			title:       "Numeric port not exposed",
			pod:         buildPodWithPort("pod-1", "http", 80),
			port:        intstr.FromInt32(8080),
			expectError: true,
			expected:    intstr.NullValue,
		},
		{
			title:       "Named port",
			pod:         buildPodWithPort("pod-1", "http", 80),
			port:        intstr.FromString("http"),
			expectError: false,
			expected:    intstr.FromInt32(80),
		},
		{
			title:       "Named port not exposed port",
			pod:         buildPodWithPort("pod-1", "http", 80),
			port:        intstr.FromString("http2"),
			expectError: true,
			expected:    intstr.NullValue,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			port, err := FindPort(tc.port, tc.pod)
			if !tc.expectError && err != nil {
				t.Errorf(" failed: %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			if tc.expectError && err != nil {
				return
			}

			if tc.expected != port {
				t.Errorf("expected %q got %q", tc.expected.Str(), port.Str())
				return
			}
		})
	}
}

func Test_GetTargetPort(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title string

		service     corev1.Service
		endpoints   *corev1.Endpoints
		port        intstr.IntOrString
		expectError bool
		expected    intstr.IntOrString
	}{
		{
			title:       "Numeric service port specified",
			service:     buildServicWithPort("test-svc", "http", 8080, k8sintstr.FromInt(80)),
			port:        intstr.FromInt32(8080),
			expectError: false,
			expected:    intstr.FromInt32(80),
		},
		{
			title:       "Named service port",
			service:     buildServicWithPort("test-svc", "http", 8080, k8sintstr.FromInt(80)),
			port:        intstr.FromString("http"),
			expectError: false,
			expected:    intstr.FromInt32(80),
		},
		{
			title:       "Named target port",
			service:     buildServicWithPort("test-svc", "http", 8080, k8sintstr.FromString("http")),
			port:        intstr.FromInt32(8080),
			expectError: false,
			expected:    intstr.FromString("http"),
		},
		{
			title:       "Default port mapping",
			service:     buildServicWithPort("test-svc", "http", 8080, k8sintstr.FromInt(80)),
			port:        intstr.FromInt32(0),
			expectError: false,
			expected:    intstr.FromInt32(80),
		},
		{
			title:       "Numeric port not exposed",
			service:     buildServicWithPort("test-svc", "http", 80, k8sintstr.FromInt(80)),
			port:        intstr.FromInt32(8080),
			expectError: true,
		},
		{
			title:       "Named port not exposed",
			service:     buildServicWithPort("test-svc", "http", 80, k8sintstr.FromString("http")),
			port:        intstr.FromString("http2"),
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			port, err := GetTargetPort(tc.service, tc.port)
			if !tc.expectError && err != nil {
				t.Errorf(" failed: %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			if tc.expectError && err != nil {
				return
			}

			if tc.expected != port {
				t.Errorf("expected %q got %q", tc.expected.Str(), port.Str())
				return
			}
		})
	}
}

func Test_Sample(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		numPods     int
		count       intstr.IntOrString
		expect      int
		expectError bool
	}{
		{
			title:       "select one pod",
			numPods:     3,
			count:       intstr.FromInt32(1),
			expect:      1,
			expectError: false,
		},
		{
			title:       "select all pods",
			numPods:     3,
			count:       intstr.FromInt32(3),
			expect:      3,
			expectError: false,
		},
		{
			title:       "select too many pods",
			numPods:     3,
			count:       intstr.FromInt32(4),
			expect:      0,
			expectError: true,
		},
		{
			title:       "select 25% of pods",
			numPods:     3,
			count:       intstr.FromString("25%"),
			expect:      1,
			expectError: false,
		},
		{
			title:       "select 30% of pods",
			numPods:     3,
			count:       intstr.FromString("30%"),
			expect:      1,
			expectError: false,
		},
		{
			title:       "select 50% of pods",
			numPods:     3,
			count:       intstr.FromString("50%"),
			expect:      2,
			expectError: false,
		},
		{
			title:       "select 100% of pods",
			numPods:     3,
			count:       intstr.FromString("100%"),
			expect:      3,
			expectError: false,
		},
		{
			title:       "select 0% of pods",
			numPods:     3,
			count:       intstr.FromString("0%"),
			expect:      0,
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			pods := []corev1.Pod{}
			for i := range tc.numPods {
				podName := fmt.Sprintf("pod-%d", i)
				pods = append(pods, builders.NewPodBuilder(podName).Build())
			}

			sample, err := Sample(pods, tc.count)

			if err != nil && !tc.expectError {
				t.Fatalf("failed %v", err)
			}

			if err == nil && tc.expectError {
				t.Fatalf("should had failed")
			}

			if err != nil && tc.expectError {
				return
			}

			if len(sample) != tc.expect {
				t.Fatalf("expected %d pods got %d", tc.expect, len(sample))
			}
		})
	}
}
