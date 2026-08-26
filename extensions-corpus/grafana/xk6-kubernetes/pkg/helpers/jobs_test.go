package helpers

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/xk6-kubernetes/pkg/resources"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stest "k8s.io/client-go/testing"

	"github.com/grafana/xk6-kubernetes/internal/testutils"
)

const (
	jobName = "test-job"
)

func newJob(name string, namespace string) *batchv1.Job {
	return &batchv1.Job{
		TypeMeta: metaV1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "xk6-kubernetes/unit-test",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: nil,
			Template:     corev1.PodTemplateSpec{},
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{},
		},
	}
}

func newJobWithStatus(name string, namespace string, status string) *batchv1.Job {
	job := newJob(name, namespace)
	job.Status.Conditions = []batchv1.JobCondition{
		{
			Type:   batchv1.JobConditionType(status),
			Status: corev1.ConditionTrue,
		},
	}
	return job
}

func TestWaitJobCompleted(t *testing.T) {
	t.Parallel()
	type TestCase struct {
		test           string
		status         string
		delay          time.Duration
		expectError    bool
		expectedResult bool
		timeout        int64
	}

	testCases := []TestCase{
		{
			test:           "job completed before timeout",
			status:         "Complete",
			delay:          1 * time.Second,
			expectError:    false,
			expectedResult: true,
			timeout:        60,
		},
		{
			test:           "timeout waiting for job to complete",
			status:         "Complete",
			delay:          10 * time.Second,
			expectError:    false,
			expectedResult: false,
			timeout:        5,
		},
		{
			test:           "job failed before timeout",
			status:         "Failed",
			delay:          1 * time.Second,
			expectError:    true,
			expectedResult: false,
			timeout:        60,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			clientset, ok := testutils.NewFakeClientset().(*fake.Clientset)
			if !ok {
				t.Errorf("invalid type assertion")
			}
			watcher := watch.NewRaceFreeFake()
			clientset.PrependWatchReactor("jobs", k8stest.DefaultWatchReactor(watcher, nil))

			fake, _ := testutils.NewFakeDynamic()
			client := resources.NewFromClient(context.TODO(), fake).WithMapper(&testutils.FakeRESTMapper{})

			fixture := NewHelper(context.TODO(), clientset, client, nil, "default")
			job := newJob(jobName, "default")
			_, err := client.Structured().Create(job)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			go func(tc TestCase) {
				time.Sleep(tc.delay)
				job = newJobWithStatus(jobName, "default", tc.status)
				_, e := client.Structured().Update(job)
				if e != nil {
					t.Errorf("unexpected error: %v", e)
					return
				}
				watcher.Modify(job)
			}(tc)

			result, err := fixture.WaitJobCompleted(
				jobName,
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
