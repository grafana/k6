package helpers

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/grafana/xk6-disruptor/pkg/testutils/assertions"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
)

const (
	testNamespace = "default"
)

func TestPods_Wait(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		test           string
		name           string
		phase          corev1.PodPhase
		delay          time.Duration
		expectError    bool
		expectedResult bool
		timeout        time.Duration
	}

	testCases := []TestCase{
		{
			test:           "wait pod running",
			name:           "pod-running",
			delay:          1 * time.Second,
			phase:          corev1.PodRunning,
			expectError:    false,
			expectedResult: true,
			timeout:        5 * time.Second,
		},
		{
			test:           "timeout waiting pod running",
			name:           "pod-running",
			phase:          corev1.PodRunning,
			delay:          10 * time.Second,
			expectError:    false,
			expectedResult: false,
			timeout:        5 * time.Second,
		},
		{
			test:           "wait failed pod",
			name:           "pod-running",
			phase:          corev1.PodFailed,
			delay:          1 * time.Second,
			expectError:    true,
			expectedResult: false,
			timeout:        5 * time.Second,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			// update status after delay
			observer := func(_ builders.ObjectEvent, pod *corev1.Pod) (*corev1.Pod, bool, error) {
				time.Sleep(tc.delay)
				pod.Status.Phase = tc.phase
				// update pod and stop watching updates
				return pod, false, nil
			}

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			client, err := builders.NewClientBuilder().
				WithContext(ctx).
				WithPodObserver(testNamespace, builders.ObjectEventAdded, observer).
				Build()
			if err != nil {
				t.Errorf("failed to create k8s client %v", err)
				return
			}

			pod := builders.NewPodBuilder(tc.name).WithNamespace(testNamespace).Build()
			_, err = client.CoreV1().Pods(testNamespace).Create(t.Context(), &pod, metav1.CreateOptions{})
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			h := NewPodHelper(client, nil, testNamespace)
			result, err := h.WaitPodRunning(
				t.Context(),
				tc.name,
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

func TestPods_AddEphemeralContainer(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		test        string
		podName     string
		expectError bool
		status      corev1.ContainerStatus
		options     AttachOptions
	}

	// TODO: check injecting agent when it is already present in the pod
	testCases := []TestCase{
		{
			test:        "Create ephemeral container not waiting",
			podName:     "test-pod",
			expectError: false,
			status: corev1.ContainerStatus{
				Name: "ephemeral",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{},
				},
			},
			options: AttachOptions{
				Timeout:        0,
				IgnoreIfExists: true,
			},
		},
		{
			test:        "Create ephemeral container waiting",
			podName:     "test-pod",
			expectError: false,
			status: corev1.ContainerStatus{
				Name: "ephemeral",
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{},
				},
			},
			options: AttachOptions{
				Timeout:        1 * time.Second,
				IgnoreIfExists: true,
			},
		},
		{
			test:        "Fail waiting for container",
			podName:     "test-pod",
			expectError: true,
			status: corev1.ContainerStatus{
				Name: "ephemeral",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{},
				},
			},
			options: AttachOptions{
				Timeout:        1 * time.Second,
				IgnoreIfExists: true,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			pod := builders.NewPodBuilder(tc.podName).
				WithNamespace(testNamespace).
				Build()

			// wait for pod to updated with ephemeral container and update status
			observer := func(_ builders.ObjectEvent, pod *corev1.Pod) (*corev1.Pod, bool, error) {
				if len(pod.Spec.EphemeralContainers) == 0 {
					return nil, true, nil
				}
				pod.Status.EphemeralContainerStatuses = []corev1.ContainerStatus{
					tc.status,
				}

				// update pod and stop watching updates
				return pod, false, nil
			}

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			client, err := builders.NewClientBuilder().
				WithContext(ctx).
				WithPods(pod).
				WithPodObserver(testNamespace, builders.ObjectEventModified, observer).
				Build()
			if err != nil {
				t.Errorf("failed to create k8s client %v", err)
				return
			}

			h := NewPodHelper(client, nil, testNamespace)
			err = h.AttachEphemeralContainer(
				t.Context(),
				tc.podName,
				corev1.EphemeralContainer{},
				tc.options,
			)
			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
				return
			}
		})
	}
}

func Test_ListPods(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title        string
		pods         []corev1.Pod
		namespace    string
		filter       PodFilter
		expectError  bool
		expectedPods []string
	}{
		{
			title: "No matching pod",
			pods: []corev1.Pod{
				builders.NewPodBuilder("pod-without-labels").
					WithNamespace("test-ns").
					Build(),
			},
			namespace: "test-ns",
			filter: PodFilter{
				Select: map[string]string{
					"app": "test",
				},
			},
			expectError:  false,
			expectedPods: []string{},
		},
		{
			title:     "No matching namespace",
			namespace: "test-ns",
			pods: []corev1.Pod{
				builders.NewPodBuilder("pod-with-app-label-in-another-ns").
					WithNamespace("anotherNamespace").
					WithLabel("app", "test").
					Build(),
			},
			filter: PodFilter{
				Select: map[string]string{
					"app": "test",
				},
			},
			expectError:  false,
			expectedPods: []string{},
		},
		{
			title:     "one matching pod",
			namespace: "test-ns",
			pods: []corev1.Pod{
				builders.NewPodBuilder("pod-with-app-label").
					WithNamespace("test-ns").
					WithLabel("app", "test").
					Build(),
			},
			filter: PodFilter{
				Select: map[string]string{
					"app": "test",
				},
			},
			expectError: false,
			expectedPods: []string{
				"pod-with-app-label",
			},
		},
		{
			title:     "multiple matching pods",
			namespace: "test-ns",
			pods: []corev1.Pod{
				builders.NewPodBuilder("pod-with-app-label").
					WithNamespace("test-ns").
					WithLabel("app", "test").
					Build(),
				builders.NewPodBuilder("another-pod-with-app-label").
					WithNamespace("test-ns").
					WithLabel("app", "test").
					Build(),
			},
			filter: PodFilter{
				Select: map[string]string{
					"app": "test",
				},
			},
			expectError: false,
			expectedPods: []string{
				"pod-with-app-label",
				"another-pod-with-app-label",
			},
		},
		{
			title:     "multiple selector labels",
			namespace: "test-ns",
			pods: []corev1.Pod{
				builders.NewPodBuilder("pod-with-app-label").
					WithNamespace("test-ns").
					WithLabel("app", "test").
					Build(),
				builders.NewPodBuilder("pod-with-dev-label").
					WithNamespace("test-ns").
					WithLabel("app", "test").
					WithLabel("env", "dev").
					Build(),
				builders.NewPodBuilder("pod-with-prod-label").
					WithNamespace("test-ns").
					WithLabel("app", "test").
					WithLabel("env", "prod").
					Build(),
			},
			filter: PodFilter{
				Select: map[string]string{
					"app": "test",
					"env": "dev",
				},
			},
			expectError: false,
			expectedPods: []string{
				"pod-with-dev-label",
			},
		},
		{
			title:     "exclude labels",
			namespace: "test-ns",
			pods: []corev1.Pod{
				builders.NewPodBuilder("pod-with-dev-label").
					WithNamespace("test-ns").
					WithLabel("app", "test").
					WithLabel("env", "dev").
					Build(),
				builders.NewPodBuilder("pod-with-prod-label").
					WithNamespace("test-ns").
					WithLabel("app", "test").
					WithLabel("env", "prod").
					Build(),
			},
			filter: PodFilter{
				Select: map[string]string{
					"app": "test",
				},
				Exclude: map[string]string{
					"env": "prod",
				},
			},
			expectError: false,
			expectedPods: []string{
				"pod-with-dev-label",
			},
		},
		{
			title:     "Namespace selector",
			namespace: "test-ns",
			pods: []corev1.Pod{
				builders.NewPodBuilder("pod-in-test-ns").
					WithNamespace("test-ns").
					Build(),
				builders.NewPodBuilder("another-pod-in-test-ns").
					WithNamespace("test-ns").
					Build(),
				builders.NewPodBuilder("pod-in-another-namespace").
					WithNamespace("other-ns").
					Build(),
			},
			filter:      PodFilter{},
			expectError: false,
			expectedPods: []string{
				"pod-in-test-ns",
				"another-pod-in-test-ns",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			pods := []runtime.Object{}
			for p := range tc.pods {
				pods = append(pods, &tc.pods[p])
			}
			client := fake.NewSimpleClientset(pods...)

			helper := NewPodHelper(client, nil, tc.namespace)
			podList, err := helper.List(t.Context(), tc.filter)

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
				return
			}

			if tc.expectError && err != nil {
				return
			}

			names := []string{}
			for _, p := range podList {
				names = append(names, p.Name)
			}
			if !assertions.CompareStringArrays(names, tc.expectedPods) {
				t.Errorf("result does not match expected value. Expected: %s\nActual: %s\n", tc.expectedPods, names)
				return
			}
		})
	}
}

func Test_WaitPodDeleted(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		namespace   string
		pods        []corev1.Pod
		target      string
		timeout     time.Duration
		delay       time.Duration
		expected    []string
		expectError bool
	}{
		{
			title:     "terminate pod",
			namespace: "test-ns",
			pods: []corev1.Pod{
				builders.NewPodBuilder("pod-1").
					WithNamespace("test-ns").
					Build(),
				builders.NewPodBuilder("pod-2").
					WithNamespace("test-ns").
					Build(),
				builders.NewPodBuilder("pod-3").
					WithNamespace("test-ns").
					Build(),
			},
			target:      "pod-1",
			timeout:     1 * time.Second,
			delay:       0,
			expectError: false,
		},
		{
			title:     "pod does not exist",
			namespace: "test-ns",
			pods: []corev1.Pod{
				builders.NewPodBuilder("pod-2").
					WithNamespace("test-ns").
					Build(),
				builders.NewPodBuilder("pod-3").
					WithNamespace("test-ns").
					Build(),
			},
			target:      "pod-1",
			timeout:     1 * time.Second,
			delay:       0,
			expectError: false,
		},
		{
			title:     "timeout",
			namespace: "test-ns",
			pods: []corev1.Pod{
				builders.NewPodBuilder("pod-1").
					WithNamespace("test-ns").
					Build(),
				builders.NewPodBuilder("pod-2").
					WithNamespace("test-ns").
					Build(),
				builders.NewPodBuilder("pod-3").
					WithNamespace("test-ns").
					Build(),
			},
			target:      "pod-1",
			timeout:     1 * time.Second,
			delay:       2 * time.Second,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			client, err := builders.NewClientBuilder().
				WithContext(ctx).
				WithPods(tc.pods...).
				Build()
			if err != nil {
				t.Errorf("failed to create k8s client %v", err)
				return
			}

			// delete pod after delay
			go func() {
				time.Sleep(tc.delay)
				err2 := client.CoreV1().Pods(tc.namespace).Delete(t.Context(), tc.target, metav1.DeleteOptions{})
				if err2 != nil && !errors.IsNotFound(err2) {
					t.Logf("deleting pod %v", err2)
				}
			}()

			helper := NewPodHelper(client, nil, tc.namespace)

			err = helper.WaitPodDeleted(t.Context(), tc.target, tc.timeout)

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
				return
			}

			if tc.expectError && err != nil {
				return
			}
		})
	}
}
