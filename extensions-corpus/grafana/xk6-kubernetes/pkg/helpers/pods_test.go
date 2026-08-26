package helpers

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/xk6-kubernetes/internal/testutils"
	"github.com/grafana/xk6-kubernetes/pkg/resources"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	podName       = "test-pod"
	testNamespace = "ns-test"
)

func buildPod() corev1.Pod {
	return corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: testNamespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sh", "-c", "sleep 300"},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}
}

func TestPods_Wait(t *testing.T) {
	t.Parallel()
	type TestCase struct {
		test           string
		status         corev1.PodPhase
		delay          time.Duration
		expectError    bool
		expectedResult bool
		timeout        int64
	}

	testCases := []TestCase{
		{
			test:           "wait pod running",
			delay:          1 * time.Second,
			status:         corev1.PodRunning,
			expectError:    false,
			expectedResult: true,
			timeout:        5,
		},
		{
			test:           "timeout waiting pod running",
			status:         corev1.PodRunning,
			delay:          10 * time.Second,
			expectError:    false,
			expectedResult: false,
			timeout:        5,
		},
		{
			test:           "wait failed pod",
			status:         corev1.PodFailed,
			delay:          1 * time.Second,
			expectError:    true,
			expectedResult: false,
			timeout:        5,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()
			fake, _ := testutils.NewFakeDynamic()
			client := resources.NewFromClient(context.TODO(), fake).WithMapper(&testutils.FakeRESTMapper{})
			clientset := testutils.NewFakeClientset()
			h := NewHelper(context.TODO(), clientset, client, nil, testNamespace)
			pod := buildPod()
			_, err := client.Structured().Create(pod)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			go func(tc TestCase) {
				pod.Status.Phase = tc.status
				time.Sleep(tc.delay)
				_, e := client.Structured().Update(pod)
				if e != nil {
					t.Errorf("unexpected error: %v", e)
					return
				}
			}(tc)

			result, err := h.WaitPodRunning(
				podName,
				tc.timeout,
			)

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tc.expectError && err == nil {
				t.Error("expected an error but none returned")
				return
			}
			if result != tc.expectedResult {
				t.Errorf("expected result %t but %t returned", tc.expectedResult, result)
				return
			}
		})
	}
}
