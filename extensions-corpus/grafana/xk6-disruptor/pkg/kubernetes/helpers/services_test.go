package helpers

import (
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/testutils/assertions"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_WaitServiceReady(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		test        string
		delay       time.Duration
		endpoints   *corev1.Endpoints
		updated     *corev1.Endpoints
		expectError bool
		timeout     time.Duration
	}

	testCases := []TestCase{
		{
			test:        "service does not exists",
			endpoints:   nil,
			updated:     nil,
			delay:       time.Second * 0,
			expectError: true,
			timeout:     time.Second * 5,
		},
		{
			test: "endpoint not created",
			endpoints: builders.NewEndPointsBuilder("service").
				BuildAsPtr(),
			updated:     nil,
			delay:       time.Second * 0,
			expectError: true,
			timeout:     time.Second * 5,
		},
		{
			test: "endpoint ready",
			endpoints: builders.NewEndPointsBuilder("service").
				WithSubset("http", 80, []string{"pod1"}).
				BuildAsPtr(),
			updated:     nil,
			delay:       time.Second * 0,
			expectError: false,
			timeout:     time.Second * 5,
		},
		{
			test:      "wait for endpoint to be ready",
			endpoints: builders.NewEndPointsBuilder("service").BuildAsPtr(),
			updated: builders.NewEndPointsBuilder("service").
				WithSubset("http", 80, []string{"pod1"}).
				BuildAsPtr(),
			delay:       time.Second * 2,
			expectError: false,
			timeout:     time.Second * 5,
		},
		{
			test:      "not ready addresses",
			endpoints: builders.NewEndPointsBuilder("service").BuildAsPtr(),
			updated: builders.NewEndPointsBuilder("service").
				WithNotReadyAddresses("http", 80, []string{"pod1"}).
				BuildAsPtr(),
			delay:       time.Second * 2,
			expectError: true,
			timeout:     time.Second * 5,
		},
		{
			test:      "timeout waiting for addresses",
			endpoints: builders.NewEndPointsBuilder("service").BuildAsPtr(),
			updated: builders.NewEndPointsBuilder("service").
				WithSubset("http", 80, []string{"pod1"}).
				BuildAsPtr(),
			delay:       time.Second * 10,
			expectError: true,
			timeout:     time.Second * 5,
		},
		{
			test: "other endpoint ready",
			endpoints: builders.NewEndPointsBuilder("another-service").
				WithSubset("http", 80, []string{"pod1"}).
				BuildAsPtr(),
			updated:     nil,
			delay:       time.Second * 10,
			expectError: true,
			timeout:     time.Second * 5,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			client := fake.NewSimpleClientset()
			if tc.endpoints != nil {
				_, err := client.CoreV1().
					Endpoints("default").
					Create(
						t.Context(),
						tc.endpoints,
						metav1.CreateOptions{},
					)
				if err != nil {
					t.Errorf("error updating endpoint: %v", err)
				}
			}

			go func(tc TestCase) {
				if tc.updated == nil {
					return
				}
				time.Sleep(tc.delay)
				_, err := client.CoreV1().
					Endpoints("default").
					Update(
						t.Context(),
						tc.updated,
						metav1.UpdateOptions{},
					)
				if err != nil {
					t.Errorf("error updating endpoint: %v", err)
				}
			}(tc)

			h := NewServiceHelper(client, "default")

			err := h.WaitServiceReady(t.Context(), "service", tc.timeout)
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tc.expectError && err == nil {
				t.Error("expected an error but none returned")
				return
			}
			// error expected and returned, it is ok
			if tc.expectError && err != nil {
				return
			}
		})
	}
}

func Test_WaitIngressReady(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		test        string
		delay       time.Duration
		ingress     *networking.Ingress
		expectError bool
		timeout     time.Duration
	}

	testCases := []TestCase{
		{
			test:        "ingress not created",
			ingress:     nil,
			delay:       time.Second * 0,
			expectError: true,
			timeout:     time.Second * 5,
		},
		{
			test: "ingress ready",
			ingress: builders.NewIngressBuilder("ingress", intstr.FromInt(80)).
				WithAddress("loadbalancer").
				BuildAsPtr(),
			delay:       time.Second * 0,
			expectError: false,
			timeout:     time.Second * 5,
		},
		{
			test:        "wait for ingress to be ready",
			ingress:     builders.NewIngressBuilder("ingress", intstr.FromInt(80)).BuildAsPtr(),
			delay:       time.Second * 2,
			expectError: false,
			timeout:     time.Second * 5,
		},
		{
			test:        "timeout waiting",
			ingress:     builders.NewIngressBuilder("ingress", intstr.FromInt(80)).BuildAsPtr(),
			delay:       time.Second * 10,
			expectError: true,
			timeout:     time.Second * 5,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()
			client := fake.NewSimpleClientset()

			if tc.ingress != nil {
				_, err := client.NetworkingV1().
					Ingresses("default").
					Create(t.Context(), tc.ingress, metav1.CreateOptions{})
				if err != nil {
					t.Errorf("error updating ingress: %v", err)
				}
			}

			// update ingress with address
			go func(tc TestCase) {
				if tc.ingress == nil {
					return
				}
				time.Sleep(tc.delay)
				updated := tc.ingress.DeepCopy()
				updated.Status.LoadBalancer.Ingress = []networking.IngressLoadBalancerIngress{
					{
						Hostname: "loadbalancer",
					},
				}
				_, err := client.NetworkingV1().
					Ingresses("default").
					Update(t.Context(), updated, metav1.UpdateOptions{})
				if err != nil {
					t.Errorf("error updating ingress: %v", err)
				}
			}(tc)

			h := NewServiceHelper(client, "default")

			err := h.WaitIngressReady(t.Context(), "ingress", tc.timeout)
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tc.expectError && err == nil {
				t.Error("expected an error but none returned")
				return
			}
			// error expected and returned, it is ok
			if tc.expectError && err != nil {
				return
			}
		})
	}
}

func Test_Targets(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title        string
		serviceName  string
		namespace    string
		service      corev1.Service
		pods         []corev1.Pod
		expectError  bool
		expectedPods []string
	}{
		{
			title:       "one endpoint",
			serviceName: "test-svc",
			namespace:   "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelectorLabel("app", "test").
				WithPort("http", 8080, intstr.FromInt(80)).
				Build(),
			pods: []corev1.Pod{
				builders.NewPodBuilder("pod-1").
					WithNamespace("test-ns").
					WithLabel("app", "test").
					Build(),
			},
			expectError:  false,
			expectedPods: []string{"pod-1"},
		},
		{
			title:       "no targets",
			serviceName: "test-svc",
			namespace:   "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelectorLabel("app", "test").
				WithPort("http", 8080, intstr.FromInt(80)).
				Build(),
			pods:         []corev1.Pod{},
			expectError:  false,
			expectedPods: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			client := fake.NewSimpleClientset()
			_, err := client.CoreV1().
				Services(tc.service.Namespace).
				Create(
					t.Context(),
					&tc.service,
					metav1.CreateOptions{},
				)
			if err != nil {
				t.Errorf("error creating service: %v", err)
			}

			for p := range tc.pods {
				_, err = client.CoreV1().
					Pods(tc.namespace).
					Create(
						t.Context(),
						&tc.pods[p],
						metav1.CreateOptions{},
					)
				if err != nil {
					t.Errorf("error creating endpoint: %v", err)
				}
			}

			helper := NewServiceHelper(client, tc.namespace)
			targets, err := helper.GetTargets(t.Context(), tc.serviceName)
			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
				return
			}

			if !tc.expectError && err != nil {
				t.Errorf(" unexpected error creating service disruptor: %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed creating service disruptor")
				return
			}

			if tc.expectError && err != nil {
				return
			}

			names := []string{}
			for _, p := range targets {
				names = append(names, p.Name)
			}
			if !assertions.CompareStringArrays(tc.expectedPods, names) {
				t.Errorf("result does not match expected value. Expected: %s\nActual: %s\n", tc.expectedPods, names)
				return
			}
		})
	}
}
