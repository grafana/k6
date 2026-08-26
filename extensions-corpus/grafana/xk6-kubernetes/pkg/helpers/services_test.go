package helpers

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/xk6-kubernetes/internal/testutils"
	"github.com/grafana/xk6-kubernetes/pkg/resources"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func buildEndpointsWithoutAddresses() *corev1.Endpoints {
	return &corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Endpoints",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service",
			Namespace: "default",
		},
		Subsets: []corev1.EndpointSubset{},
	}
}

func buildEndpointsWithAddresses() *corev1.Endpoints {
	return &corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Endpoints",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service",
			Namespace: "default",
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: "1.1.1.1",
					},
				},
			},
		},
	}
}

func buildOtherEndpointsWithAddresses() *corev1.Endpoints {
	return &corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Endpoints",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otherservice",
			Namespace: "default",
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: "1.1.1.1",
					},
				},
			},
		},
	}
}

func buildEndpointsWithNotReadyAddresses() *corev1.Endpoints {
	return &corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Endpoints",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service",
			Namespace: "default",
		},
		Subsets: []corev1.EndpointSubset{
			{
				NotReadyAddresses: []corev1.EndpointAddress{
					{
						IP: "1.1.1.1",
					},
				},
			},
		},
	}
}

func buildService() corev1.Service {
	return corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{},
			},
		},
	}
}

func Test_WaitServiceReady(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		test          string
		delay         time.Duration
		endpoints     *corev1.Endpoints
		updated       *corev1.Endpoints
		expectedValue bool
		expectError   bool
		timeout       int64
	}

	testCases := []TestCase{
		{
			test:          "endpoint not created",
			endpoints:     nil,
			updated:       nil,
			delay:         time.Second * 0,
			expectedValue: false,
			expectError:   false,
			timeout:       5,
		},
		{
			test:          "endpoint already ready",
			endpoints:     buildEndpointsWithAddresses(),
			updated:       nil,
			delay:         time.Second * 0,
			expectedValue: true,
			expectError:   false,
			timeout:       5,
		},
		{
			test:          "wait for endpoint to be ready",
			endpoints:     buildEndpointsWithoutAddresses(),
			updated:       buildEndpointsWithAddresses(),
			delay:         time.Second * 2,
			expectedValue: true,
			expectError:   false,
			timeout:       5,
		},
		{
			test:          "not ready addresses",
			endpoints:     buildEndpointsWithoutAddresses(),
			updated:       buildEndpointsWithNotReadyAddresses(),
			delay:         time.Second * 2,
			expectedValue: false,
			expectError:   false,
			timeout:       5,
		},
		{
			test:          "timeout waiting for addresses",
			endpoints:     buildEndpointsWithoutAddresses(),
			updated:       buildEndpointsWithAddresses(),
			delay:         time.Second * 10,
			expectedValue: false,
			expectError:   false,
			timeout:       5,
		},
		{
			test:          "other endpoint ready",
			endpoints:     buildOtherEndpointsWithAddresses(),
			updated:       nil,
			delay:         time.Second * 10,
			expectedValue: false,
			expectError:   false,
			timeout:       5,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()
			objs := []runtime.Object{}
			if tc.endpoints != nil {
				objs = append(objs, tc.endpoints)
			}
			fake, _ := testutils.NewFakeDynamic(objs...)
			client := resources.NewFromClient(context.TODO(), fake).WithMapper(&testutils.FakeRESTMapper{})
			clientset := testutils.NewFakeClientset()
			h := NewHelper(context.TODO(), clientset, client, nil, "default")

			go func(tc TestCase) {
				if tc.updated == nil {
					return
				}
				time.Sleep(tc.delay)

				_, e := client.Structured().Update(tc.updated)
				if e != nil {
					t.Errorf("error updating endpoint: %v", e)
				}
			}(tc)

			ready, err := h.WaitServiceReady("service", tc.timeout)
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tc.expectError && err == nil {
				t.Error("expected an error but none returned")
				return
			}

			if ready != tc.expectedValue {
				t.Errorf("invalid value returned expected %t actual %t", tc.expectedValue, ready)
				return
			}
		})
	}
}

func Test_GetServiceIP(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		test          string
		delay         time.Duration
		updated       []corev1.LoadBalancerIngress
		expectedValue string
		expectError   bool
		timeout       int64
	}

	testCases := []TestCase{
		{
			test: "wait for ip to be assigned",
			updated: []corev1.LoadBalancerIngress{
				{
					IP: "1.1.1.1",
				},
			},
			delay:         time.Second * 2,
			expectedValue: "1.1.1.1",
			expectError:   false,
			timeout:       5,
		},
		{
			test:          "timeout waiting for addresses",
			updated:       []corev1.LoadBalancerIngress{},
			delay:         time.Second * 10,
			expectedValue: "",
			expectError:   false,
			timeout:       5,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			fake, _ := testutils.NewFakeDynamic()
			client := resources.NewFromClient(context.TODO(), fake).WithMapper(&testutils.FakeRESTMapper{})
			clientset := testutils.NewFakeClientset()
			h := NewHelper(context.TODO(), clientset, client, nil, "default")

			svc := buildService()
			_, err := client.Structured().Create(svc)
			if err != nil {
				t.Errorf("unexpected error creating service: %v", err)
				return
			}

			go func(tc TestCase, svc corev1.Service) {
				time.Sleep(tc.delay)
				svc.Status.LoadBalancer.Ingress = tc.updated
				_, e := client.Structured().Update(svc)
				if e != nil {
					t.Errorf("error updating service: %v", e)
				}
			}(tc, svc)

			addr, err := h.GetExternalIP("service", tc.timeout)
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tc.expectError && err == nil {
				t.Error("expected an error but none returned")
				return
			}

			if addr != tc.expectedValue {
				t.Errorf("invalid value returned expected %s actual %s", tc.expectedValue, addr)
				return
			}
		})
	}
}
