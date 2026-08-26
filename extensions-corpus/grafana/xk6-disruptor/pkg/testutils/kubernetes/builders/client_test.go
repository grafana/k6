package builders

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_WithPodObservers(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		test          string
		event         ObjectEvent
		setup         func(client *fake.Clientset) error
		obsErr        error
		obsBool       bool
		expectErr     bool
		expectTimeout bool
	}{
		{
			test:          "Observe Pod Added",
			event:         ObjectEventAdded,
			obsErr:        nil,
			obsBool:       false,
			expectErr:     false,
			expectTimeout: false,
			setup: func(client *fake.Clientset) error {
				pod := NewPodBuilder("pod1").WithNamespace("default").Build()
				_, err := client.CoreV1().Pods("default").Create(t.Context(), &pod, metav1.CreateOptions{})
				return err
			},
		},
		{
			test:          "Observe Pod Modified",
			event:         ObjectEventModified,
			obsErr:        nil,
			obsBool:       false,
			expectErr:     false,
			expectTimeout: false,
			setup: func(client *fake.Clientset) error {
				pod := NewPodBuilder("pod1").WithNamespace("default").WithAnnotation("test.updated", "").Build()

				created, err := client.CoreV1().Pods("default").Create(t.Context(), &pod, metav1.CreateOptions{})
				if err != nil {
					return err
				}

				time.Sleep(time.Second)

				created.Annotations["test.updated"] = "true"
				_, err = client.CoreV1().Pods("default").Update(t.Context(), created, metav1.UpdateOptions{})
				return err
			},
		},
		{
			test:          "Observe Pod Modified timeout",
			event:         ObjectEventModified,
			obsErr:        nil,
			obsBool:       false,
			expectErr:     false,
			expectTimeout: true,
			setup: func(client *fake.Clientset) error {
				pod := NewPodBuilder("pod1").WithNamespace("default").Build()
				_, err := client.CoreV1().Pods("default").Create(t.Context(), &pod, metav1.CreateOptions{})
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			// test setup errors
			errCh := make(chan error)
			// observer termination signal
			obsCh := make(chan bool)
			// observer errors
			obsErr := make(chan error)
			// observer cancellation context
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			client, err := NewClientBuilder().
				WithErrorChannel(obsErr).
				WithContext(ctx).
				WithPodObserver(
					"default",
					tc.event,
					func(action ObjectEvent, pod *corev1.Pod) (*corev1.Pod, bool, error) { //nolint:revive
						if tc.event != ObjectEventAll && action != tc.event {
							return nil, false, fmt.Errorf("invalid action. Expected: %s got %s", tc.event, action)
						}

						// signal observer has been executed
						if tc.obsErr == nil {
							obsCh <- true
						}

						return nil, tc.obsBool, tc.obsErr
					},
				).
				Build()
			if err != nil {
				t.Errorf("failed to create client %v", err)
				return
			}

			// execute the test setup in background
			go func() {
				e := tc.setup(client)
				if e != nil {
					errCh <- fmt.Errorf("test setup failed %w", e)
				}
			}()

			timer := time.NewTimer(3 * time.Second)
			select {
			case <-obsCh:
				// observer signaled execution without errors
				return
			case <-timer.C:
				if !tc.expectTimeout {
					t.Errorf("test timeout")
				}
				return
			case err = <-errCh:
				t.Errorf("unexpected error: %v", err)
			case err = <-obsErr:
				// observer signaled error, check if it is expected
				if !tc.expectErr {
					t.Errorf("failed with error: %v", err)
				}
			}
		})
	}
}
