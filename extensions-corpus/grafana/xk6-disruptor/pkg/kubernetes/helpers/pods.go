package helpers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// PodHelper defines helper methods for handling Pods
type PodHelper interface {
	// WaitPodRunning waits for the Pod to be running for up to given timeout and returns a boolean indicating
	// if the status was reached. If the pod is Failed returns error.
	WaitPodRunning(ctx context.Context, name string, timeout time.Duration) (bool, error)
	// WaitPodDeleted waits for the Pod to be deleted for up to given timeout
	WaitPodDeleted(ctx context.Context, name string, timeout time.Duration) error
	// Exec executes a non-interactive command described in options and returns the stdout and stderr outputs
	Exec(ctx context.Context, pod string, container string, command []string, stdin []byte) ([]byte, []byte, error)
	// AttachEphemeralContainer adds an ephemeral container to a running pod
	AttachEphemeralContainer(
		ctx context.Context,
		podName string,
		container corev1.EphemeralContainer,
		options AttachOptions,
	) error
	// List returns a list of pods that match the given PodFilter
	List(ctx context.Context, filter PodFilter) ([]corev1.Pod, error)
	// Terminate terminates the execution of a running Pod
	Terminate(ctx context.Context, name string, timeout time.Duration) error
}

// helpers struct holds the data required by the helpers
type podHelper struct {
	client    kubernetes.Interface
	executor  PodCommandExecutor
	namespace string
}

// NewPodHelper returns a PodHelper
func NewPodHelper(client kubernetes.Interface, executor PodCommandExecutor, namespace string) PodHelper {
	return &podHelper{
		client:    client,
		namespace: namespace,
		executor:  executor,
	}
}

// PodFilter defines the criteria for selecting a pod for disruption
type PodFilter struct {
	// Select Pods that match these labels
	Select map[string]string
	// Select Pods that match these labels
	Exclude map[string]string
}

// AttachOptions defines options for attaching a container
type AttachOptions struct {
	// timeout for waiting until container is ready.
	Timeout time.Duration
	// IgnoreIfExists causes AttachEphemeralContainer to return successfully if the ephemeral container already exists
	// when set to true. If set to false, it will exit with an error if the container already exists.
	IgnoreIfExists bool
}

// podConditionChecker defines a function that checks if a pod satisfies a condition
type podConditionChecker func(*corev1.Pod) (bool, error)

// waitForCondition watches a Pod in a namespace until a podConditionChecker is satisfied or a timeout expires
func (h *podHelper) waitForCondition(
	ctx context.Context,
	namespace string,
	name string,
	timeout time.Duration,
	checker podConditionChecker,
) (bool, error) {
	selector := fields.Set{
		"metadata.name": name,
	}.AsSelector()

	watcher, err := h.client.CoreV1().Pods(namespace).Watch(
		ctx,
		metav1.ListOptions{
			FieldSelector: selector.String(),
		},
	)
	if err != nil {
		return false, fmt.Errorf("starting pod watcher: %w", err)
	}
	defer watcher.Stop()

	// we check if the pod already satisfies the condition to prevent race conditions
	// on which we miss the update that makes the condition true
	pod, err := h.client.CoreV1().Pods(namespace).Get(
		ctx,
		name,
		metav1.GetOptions{},
	)
	if err != nil {
		return false, fmt.Errorf("getting pod: %w", err)
	}

	condition, err := checker(pod)
	if condition || err != nil {
		return condition, err
	}

	expired := time.After(timeout)
	for {
		select {
		case <-expired:
			return false, nil
		case event := <-watcher.ResultChan():
			if event.Type == watch.Error {
				return false, fmt.Errorf("error watching for pod: %v", event.Object)
			}
			if event.Type == watch.Modified {
				pod, isPod := event.Object.(*corev1.Pod)
				if !isPod {
					return false, errors.New("received unknown object while watching for pods")
				}
				condition, err = checker(pod)
				if condition || err != nil {
					return condition, err
				}
			}
		}
	}
}

func (h *podHelper) WaitPodRunning(ctx context.Context, name string, timeout time.Duration) (bool, error) {
	return h.waitForCondition(
		ctx,
		h.namespace,
		name,
		timeout,
		func(pod *corev1.Pod) (bool, error) {
			if pod.Status.Phase == corev1.PodFailed {
				return false, errors.New("pod has failed")
			}
			if pod.Status.Phase == corev1.PodRunning {
				return true, nil
			}
			return false, nil
		},
	)
}

func (h *podHelper) Exec(
	ctx context.Context,
	pod string,
	container string,
	command []string,
	stdin []byte,
) ([]byte, []byte, error) {
	return h.executor.Exec(
		ctx,
		pod,
		h.namespace,
		container,
		command,
		stdin,
	)
}

func (h *podHelper) AttachEphemeralContainer(
	ctx context.Context,
	podName string,
	container corev1.EphemeralContainer,
	options AttachOptions,
) error {
	pod, err := h.client.CoreV1().Pods(h.namespace).Get(
		ctx,
		podName,
		metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("retrieving pod %q in %q: %w", podName, h.namespace, err)
	}

	// check if container already exists
	for _, c := range pod.Spec.EphemeralContainers {
		if c.Name == container.Name {
			if options.IgnoreIfExists {
				return nil
			}
			return fmt.Errorf("ephemeral container %s already exists", container.Name)
		}
	}

	podJSON, err := json.Marshal(pod)
	if err != nil {
		return fmt.Errorf("json marshalling pod %q: %w", pod.Name, err)
	}

	updatedPod := pod.DeepCopy()
	updatedPod.Spec.EphemeralContainers = append(updatedPod.Spec.EphemeralContainers, container)
	updateJSON, err := json.Marshal(updatedPod)
	if err != nil {
		return fmt.Errorf("json marshalling patched pod %q: %w", pod.Name, err)
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(podJSON, updateJSON, pod)
	if err != nil {
		return fmt.Errorf("creating ephemeral container patch for %q: %w", pod.Name, err)
	}

	_, err = h.client.CoreV1().Pods(h.namespace).Patch(
		ctx,
		pod.Name,
		types.StrategicMergePatchType,
		patch,
		metav1.PatchOptions{},
		"ephemeralcontainers",
	)
	if err != nil {
		return fmt.Errorf("patching ephemeral container into pod %q: %w", pod.Name, err)
	}

	if options.Timeout == 0 {
		return nil
	}
	running, err := h.waitForCondition(
		ctx,
		h.namespace,
		podName,
		options.Timeout,
		checkEphemeralContainerIsRunning,
	)
	if err != nil {
		return fmt.Errorf("waiting for ephemeral container of %q to start: %w", pod.Name, err)
	}
	if !running {
		return fmt.Errorf("ephemeral container for pod %q has not started after %fs", pod.Name, options.Timeout.Seconds())
	}
	return nil
}

func checkEphemeralContainerIsRunning(pod *corev1.Pod) (bool, error) {
	if pod.Status.EphemeralContainerStatuses != nil {
		for _, cs := range pod.Status.EphemeralContainerStatuses {
			if cs.State.Running != nil {
				return true, nil
			}
		}
	}

	return false, nil
}

// buildLabelSelector builds a label selector to be used in the k8s api, from a PodSelector
func buildLabelSelector(f PodFilter) (labels.Selector, error) {
	labelsSelector := labels.NewSelector()
	for label, value := range f.Select {
		req, err := labels.NewRequirement(label, selection.Equals, []string{value})
		if err != nil {
			return nil, err
		}
		labelsSelector = labelsSelector.Add(*req)
	}

	for label, value := range f.Exclude {
		req, err := labels.NewRequirement(label, selection.NotEquals, []string{value})
		if err != nil {
			return nil, err
		}
		labelsSelector = labelsSelector.Add(*req)
	}

	return labelsSelector, nil
}

func (h *podHelper) List(ctx context.Context, filter PodFilter) ([]corev1.Pod, error) {
	labelSelector, err := buildLabelSelector(filter)
	if err != nil {
		return nil, err
	}

	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	}
	pods, err := h.client.CoreV1().Pods(h.namespace).List(
		ctx,
		listOptions,
	)
	if err != nil {
		return nil, err
	}

	return pods.Items, nil
}

// WaitPodDeleted waits until a pod is deleted or a timeout expires
func (h *podHelper) WaitPodDeleted(ctx context.Context, pod string, timeout time.Duration) error {
	selector := fields.Set{
		"metadata.name": pod,
	}.AsSelector()

	watcher, err := h.client.CoreV1().Pods(h.namespace).Watch(
		ctx,
		metav1.ListOptions{
			FieldSelector: selector.String(),
		},
	)
	if err != nil {
		return fmt.Errorf("starting pod watcher: %w", err)
	}
	defer watcher.Stop()

	// if pod does not exist, then consider it deleted
	_, err = h.client.CoreV1().Pods(h.namespace).Get(
		ctx,
		pod,
		metav1.GetOptions{},
	)
	if k8serrors.IsNotFound(err) {
		return nil
	}

	expired := time.After(timeout)
	for {
		select {
		case <-expired:
			return fmt.Errorf("pod '%s/%s' not terminated after %fs", h.namespace, pod, timeout.Seconds())
		case event := <-watcher.ResultChan():
			if event.Type == watch.Error {
				return fmt.Errorf("error watching for pod: %v", event.Object)
			}
			if event.Type == watch.Deleted {
				return nil
			}
		}
	}
}

// Terminate terminates a running Pod
func (h *podHelper) Terminate(ctx context.Context, pod string, timeout time.Duration) error {
	err := h.client.CoreV1().Pods(h.namespace).Delete(ctx, pod, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("deleting pod %w", err)
	}

	return h.WaitPodDeleted(ctx, pod, timeout)
}
