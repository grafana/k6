// Package fixtures implements helpers for setting e2e tests
package fixtures

import (
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// BuildHttpbinPod returns the definition for deploying Httpbin as a Pod
func BuildHttpbinPod() corev1.Pod {
	c := builders.NewContainerBuilder("httpbin").
		WithImage("kennethreitz/httpbin").
		WithPullPolicy(corev1.PullIfNotPresent).
		WithPort("http", 80).
		Build()

	return builders.NewPodBuilder("httpbin").
		WithLabel("app", "httpbin").
		WithContainer(c).
		Build()
}

// BuildGrpcpbinPod returns the definition for deploying grpcbin as a Pod
func BuildGrpcpbinPod() corev1.Pod {
	c := builders.NewContainerBuilder("grpcbin").
		WithImage("moul/grpcbin").
		WithPullPolicy(corev1.PullIfNotPresent).
		WithPort("grpc", 9000).
		Build()

	return builders.NewPodBuilder("grpcbin").
		WithLabel("app", "grpcbin").
		WithContainer(c).
		Build()
}

// BuildHttpbinService returns a Service definition that exposes httpbin pods
func BuildHttpbinService() corev1.Service {
	return builders.NewServiceBuilder("httpbin").
		WithSelectorLabel("app", "httpbin").
		WithPort("http", 80, intstr.FromString("http")).
		Build()
}

// BuildGrpcbinService returns a Service definition that exposes grpcbin pods at the node port 30000
func BuildGrpcbinService() corev1.Service {
	return builders.NewServiceBuilder("grpcbin").
		WithSelectorLabel("app", "grpcbin").
		WithServiceType(corev1.ServiceTypeClusterIP).
		WithAnnotation("projectcontour.io/upstream-protocol.h2c", "9000").
		WithPort("grpc", 9000, intstr.FromInt(9000)).
		Build()
}

// BuildBusyBoxPod returns the definition of a Pod that runs busybox and waits 5min before completing
func BuildBusyBoxPod() corev1.Pod {
	c := builders.NewContainerBuilder("busybox").
		WithImage("busybox").
		WithPullPolicy(corev1.PullIfNotPresent).
		WithCommand("sleep", "300").
		Build()

	return builders.NewPodBuilder("busybox").
		WithLabel("app", "busybox").
		WithContainer(c).
		Build()
}

// BuildPausedPod returns the definition of a Pod that runs the paused image in a container
// creating a "no-op" dummy Pod.
func BuildPausedPod() corev1.Pod {
	c := builders.NewContainerBuilder("paused").
		WithImage("k8s.gcr.io/pause").
		WithPullPolicy(corev1.PullIfNotPresent).
		Build()

	return builders.NewPodBuilder("paused").
		WithContainer(c).
		Build()
}

// BuildNginxPod returns the definition of a Pod that runs Nginx
func BuildNginxPod() corev1.Pod {
	c := builders.NewContainerBuilder("busybox").
		WithImage("nginx").
		WithPullPolicy(corev1.PullIfNotPresent).
		WithPort("http", 80).
		Build()

	return builders.NewPodBuilder("nginx").
		WithLabel("app", "nginx").
		WithContainer(c).
		Build()
}

// BuildNginxService returns the definition of a Service that exposes the nginx pod(s)
func BuildNginxService() corev1.Service {
	return builders.NewServiceBuilder("nginx").
		WithSelectorLabel("app", "nginx").
		WithPort("http", 80, intstr.FromInt(80)).
		Build()
}

// BuildEchoServerPod returns the definition for deploying echoserver as a Pod
// The image is built from the testcontainers/echoserver directory and is not published to a registry.
func BuildEchoServerPod() corev1.Pod {
	c := builders.NewContainerBuilder("echoserver").
		WithImage("xk6-disruptor-echoserver:latest").
		WithPullPolicy(corev1.PullNever).
		WithPort("tcp", 6666).
		Build()

	return builders.NewPodBuilder("echoserver").
		WithLabel("app", "echoserver").
		WithContainer(c).
		Build()
}

// BuildEchoServerService returns a Service definition that exposes echoserver pods
func BuildEchoServerService() corev1.Service {
	return builders.NewServiceBuilder("echoserver").
		WithSelectorLabel("app", "echoserver").
		WithPort("tcp", 6666, intstr.FromInt(6666)).
		Build()
}
