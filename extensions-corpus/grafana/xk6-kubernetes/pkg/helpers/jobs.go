package helpers

import (
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
)

// JobHelper defines helper functions for manipulating Jobs
type JobHelper interface {
	// WaitJobCompleted waits for the Job to be completed for up to the given timeout (in seconds) and returns
	// a boolean indicating if the status was reached. If the job is Failed an error is returned.
	WaitJobCompleted(name string, timeout int64) (bool, error)
}

// isCompleted returns if the job is completed or not. Returns an error if the job is failed.
func isCompleted(job *batchv1.Job) (bool, error) {
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			return false, fmt.Errorf("job failed with reason: %v", condition.Reason)
		}
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			return true, nil
		}
	}
	return false, nil
}

func (h *helpers) WaitJobCompleted(name string, timeout int64) (bool, error) {
	deadline := time.Duration(timeout) * time.Second
	selector := fields.Set{
		"metadata.name": name,
	}.AsSelector()
	watcher, err := h.clientset.BatchV1().Jobs(h.namespace).Watch(
		h.ctx,
		metav1.ListOptions{
			FieldSelector: selector.String(),
		},
	)
	if err != nil {
		return false, err
	}
	defer watcher.Stop()

	for {
		select {
		case <-time.After(deadline):
			return false, nil
		case event := <-watcher.ResultChan():
			if event.Type == watch.Error {
				return false, fmt.Errorf("error watching for job: %v", event.Object)
			}
			if event.Type == watch.Modified {
				job, isJob := event.Object.(*batchv1.Job)
				if !isJob {
					return false, fmt.Errorf("received unknown object while watching for jobs")
				}
				completed, jobError := isCompleted(job)
				if jobError != nil {
					return false, jobError
				}
				if completed {
					return true, nil
				}
			}
		}
	}
}
