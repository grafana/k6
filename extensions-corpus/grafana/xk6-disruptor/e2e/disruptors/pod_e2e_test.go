//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
	"github.com/grafana/xk6-disruptor/pkg/types/intstr"

	errors "errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sintstr "k8s.io/apimachinery/pkg/util/intstr"

	"github.com/grafana/xk6-disruptor/pkg/disruptors"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/checks"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/cluster"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/deploy"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/fixtures"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/kubectl"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/kubernetes/namespace"
)

func Test_PodDisruptor(t *testing.T) {
	t.Parallel()

	cluster, err := cluster.BuildE2eCluster(
		cluster.DefaultE2eClusterConfig(),
	)
	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
		return
	}
	t.Cleanup(func() {
		_ = cluster.Cleanup()
	})

	err = cluster.Load(
		fixtures.BuildHttpbinPod().Spec.Containers[0].Image,
		fixtures.BuildGrpcpbinPod().Spec.Containers[0].Image,
		fixtures.BuildEchoServerPod().Spec.Containers[0].Image,
	)
	if err != nil {
		t.Fatalf("preloading test pod images: %v", err)
	}

	k8s, err := kubernetes.NewFromKubeconfig(cluster.Kubeconfig())
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}
	t.Run("Protocol fault injection", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			title    string
			pod      corev1.Pod
			replicas int
			service  corev1.Service
			port     int
			injector func(d disruptors.PodDisruptor) error
			check    checks.Check
		}{
			{
				title:    "Inject Http error 500",
				pod:      fixtures.BuildHttpbinPod(),
				replicas: 1,
				service:  fixtures.BuildHttpbinService(),
				port:     80,
				injector: func(d disruptors.PodDisruptor) error {
					fault := disruptors.HTTPFault{
						Port:      intstr.FromInt32(80),
						ErrorRate: 1.0,
						ErrorCode: 500,
					}
					options := disruptors.HTTPDisruptionOptions{
						ProxyPort: 8080,
					}

					return d.InjectHTTPFaults(t.Context(), fault, 10*time.Second, options)
				},
				check: checks.HTTPCheck{
					Service:      "httpbin",
					Method:       "GET",
					Path:         "/status/200",
					Body:         []byte{},
					ExpectedCode: 500,
					Delay:        5 * time.Second,
				},
			},
			{
				title:    "Inject Grpc error",
				pod:      fixtures.BuildGrpcpbinPod(),
				replicas: 1,
				service:  fixtures.BuildGrpcbinService(),
				port:     9000,
				injector: func(d disruptors.PodDisruptor) error {
					fault := disruptors.GrpcFault{
						Port:       intstr.FromInt32(9000),
						ErrorRate:  1.0,
						StatusCode: 14,
						Exclude:    "grpc.reflection.v1alpha.ServerReflection,grpc.reflection.v1.ServerReflection",
					}
					options := disruptors.GrpcDisruptionOptions{
						ProxyPort: 3000,
					}

					return d.InjectGrpcFaults(t.Context(), fault, 10*time.Second, options)
				},
				check: checks.GrpcCheck{
					Service:        "grpcbin",
					GrpcService:    "grpcbin.GRPCBin",
					Method:         "Empty",
					Request:        []byte("{}"),
					ExpectedStatus: 14,
					Delay:          5 * time.Second,
				},
			},
			{
				title:    "Network fault injection",
				pod:      fixtures.BuildEchoServerPod(),
				replicas: 1,
				service:  fixtures.BuildEchoServerService(),
				port:     6666,
				injector: func(d disruptors.PodDisruptor) error {
					fault := disruptors.NetworkFault{}

					return d.InjectNetworkFaults(t.Context(), fault, 1*time.Hour)
				},
				check: checks.EchoCheck{
					ExpectFailure: true,
					Delay:         5 * time.Second,
					Timeout:       5 * time.Second,
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.title, func(t *testing.T) {
				t.Parallel()

				namespace, err := namespace.CreateTestNamespace(t.Context(), t, k8s.Client())
				if err != nil {
					t.Errorf("failed to create test namespace: %v", err)
					return
				}

				err = deploy.ExposeApp(
					k8s,
					namespace,
					tc.pod,
					tc.replicas,
					tc.service,
					k8sintstr.FromInt(tc.port),
					30*time.Second,
				)
				if err != nil {
					t.Errorf("error deploying application: %v", err)
					return
				}

				// create pod disruptor that will select the service's pods
				selector := disruptors.PodSelectorSpec{
					Namespace: namespace,
					Select: disruptors.PodAttributes{
						Labels: tc.service.Spec.Selector,
					},
				}
				options := disruptors.PodDisruptorOptions{}
				disruptor, err := disruptors.NewPodDisruptor(t.Context(), k8s, selector, options)
				if err != nil {
					t.Errorf("error creating selector: %v", err)
					return
				}

				targets, _ := disruptor.Targets(t.Context())
				if len(targets) == 0 {
					t.Errorf("No pods matched the selector")
					return
				}

				// apply disruption in a go-routine as it is a blocking function
				go func() {
					err := tc.injector(disruptor)
					if err != nil && !errors.Is(err, context.Canceled) {
						t.Logf("failed to setup disruptor: %v", err)
						return
					}
				}()

				t.Run("Access via ingress", func(t *testing.T) {
					t.Parallel()

					err = tc.check.Verify(k8s, cluster.Ingress(), namespace)
					if err != nil {
						t.Errorf("failed to access service: %v", err)
						return
					}
				})

				t.Run("via port-forward", func(t *testing.T) {
					t.Parallel()

					kc, err := kubectl.NewFromKubeconfig(t.Context(), cluster.Kubeconfig())
					if err != nil {
						t.Fatalf("creating kubectl client from kubeconfig: %v", err)
					}

					if tc.port < 0 {
						t.Fatalf("negative test port: %d", tc.port)
					}
					// connect via port forwarding to the first pod in the pod set
					port, err := kc.ForwardPodPort(t.Context(), namespace, tc.pod.Name+"-0", uint(tc.port))
					if err != nil {
						t.Fatalf("forwarding port from %s/%s: %v", namespace, tc.pod.Name, err)
					}

					err = tc.check.Verify(k8s, net.JoinHostPort("localhost", fmt.Sprint(port)), namespace)
					if err != nil {
						t.Errorf("failed to access service: %v", err)
						return
					}
				})
			})
		}
	})

	t.Run("Disruptor errors out if no requests are received", func(t *testing.T) {
		t.Parallel()

		namespace, err := namespace.CreateTestNamespace(t.Context(), t, k8s.Client())
		if err != nil {
			t.Fatalf("failed to create test namespace: %v", err)
		}

		service := fixtures.BuildHttpbinService()
		err = deploy.ExposeApp(
			k8s,
			namespace,
			fixtures.BuildHttpbinPod(),
			1,
			service,
			k8sintstr.FromInt(80),
			30*time.Second,
		)
		if err != nil {
			t.Fatalf("error deploying application: %v", err)
		}

		// create pod disruptor that will select the service's pods
		selector := disruptors.PodSelectorSpec{
			Namespace: namespace,
			Select: disruptors.PodAttributes{
				Labels: service.Spec.Selector,
			},
		}
		options := disruptors.PodDisruptorOptions{}
		disruptor, err := disruptors.NewPodDisruptor(t.Context(), k8s, selector, options)
		if err != nil {
			t.Fatalf("error creating selector: %v", err)
		}

		targets, _ := disruptor.Targets(t.Context())
		if len(targets) == 0 {
			t.Fatalf("No pods matched the selector")
		}

		fault := disruptors.HTTPFault{
			Port:      intstr.FromInt32(80),
			ErrorRate: 1.0,
			ErrorCode: 500,
		}
		disruptorOptions := disruptors.HTTPDisruptionOptions{
			ProxyPort: 8080,
		}
		err = disruptor.InjectHTTPFaults(t.Context(), fault, 5*time.Second, disruptorOptions)
		if err == nil {
			t.Fatalf("disruptor did not return an error")
		}

		// It is not possible to use errors.Is here, as ErrNoRequests is returned inside the agent pod.
		// The controller only sees the error message printed to stderr.
		if !strings.Contains(err.Error(), protocol.ErrNoRequests.Error()) {
			t.Fatalf("expected ErrNoRequests, got: %v", err)
		}
	})

	t.Run("Terminate Pod", func(t *testing.T) {
		t.Parallel()

		namespace, err := namespace.CreateTestNamespace(t.Context(), t, k8s.Client())
		if err != nil {
			t.Fatalf("failed to create test namespace: %v", err)
		}

		err = deploy.RunPodSet(
			k8s,
			namespace,
			fixtures.BuildHttpbinPod(),
			3,
			30*time.Second,
		)
		if err != nil {
			t.Fatalf("starting pod replicas %v", err)
		}

		// create pod disruptor that will select all pods
		selector := disruptors.PodSelectorSpec{
			Namespace: namespace,
		}
		options := disruptors.PodDisruptorOptions{}
		disruptor, err := disruptors.NewPodDisruptor(t.Context(), k8s, selector, options)
		if err != nil {
			t.Fatalf("creating disruptor: %v", err)
		}

		targets, _ := disruptor.Targets(t.Context())
		if len(targets) == 0 {
			t.Fatalf("No pods matched the selector")
		}

		fault := disruptors.PodTerminationFault{
			Count:   intstr.FromInt32(1),
			Timeout: 10 * time.Second,
		}

		terminated, err := disruptor.TerminatePods(t.Context(), fault)
		if err != nil {
			t.Fatalf("terminating pods: %v", err)
		}

		if len(terminated) != int(fault.Count.Int32()) {
			t.Fatalf("Invalid number of pods deleted. Expected %d got %d", fault.Count.Int32(), len(terminated))
		}

		for _, pod := range terminated {
			_, err = k8s.Client().CoreV1().Pods(namespace).Get(t.Context(), pod, metav1.GetOptions{})
			if !apierrors.IsNotFound(err) {
				if err == nil {
					t.Fatalf("pod '%s/%s' not deleted", namespace, pod)
				}

				t.Fatalf("failed %v", err)
			}
		}
	})
}
