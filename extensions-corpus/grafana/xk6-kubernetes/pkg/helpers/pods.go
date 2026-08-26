package helpers

import (
	"bytes"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/grafana/xk6-kubernetes/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

// PodHelper defines helper functions for manipulating Pods
type PodHelper interface {
	// ExecuteInPod executes a non-interactive command described in options and returns the stdout and stderr outputs
	ExecuteInPod(options PodExecOptions) (*PodExecResult, error)
	// WaitPodRunning waits for the Pod to be running for up to given timeout (in seconds) and returns
	// a boolean indicating if the status was reached. If the pod is Failed returns error.
	WaitPodRunning(name string, timeout int64) (bool, error)
}

func (h *helpers) WaitPodRunning(name string, timeout int64) (bool, error) {
	return utils.Retry(time.Duration(timeout)*time.Second, time.Second, func() (bool, error) {
		pod := &corev1.Pod{}
		err := h.client.Structured().Get("Pod", name, h.namespace, pod)
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if pod.Status.Phase == corev1.PodFailed {
			return false, fmt.Errorf("pod has failed")
		}
		if pod.Status.Phase == corev1.PodRunning {
			return true, nil
		}
		return false, nil
	})
}

// PodExecOptions describe the command to be executed and the target container
type PodExecOptions struct {
	Pod       string   // name of the Pod to execute the command in
	Container string   // name of the container to execute the command in
	Command   []string // command to be executed with its parameters
	Stdin     []byte   // stdin to be supplied to the command
	Timeout   int64    // number of seconds allowed to wait for completion
}

// PodExecResult contains the output obtained from the execution of a command
type PodExecResult struct {
	Stdout []byte
	Stderr []byte
}

func (h *helpers) ExecuteInPod(options PodExecOptions) (*PodExecResult, error) {
	result := PodExecResult{}
	_, err := utils.Retry(time.Duration(options.Timeout)*time.Second, time.Second, func() (bool, error) {
		req := h.clientset.CoreV1().RESTClient().
			Post().
			Namespace(h.namespace).
			Resource("pods").
			Name(options.Pod).
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Container: options.Container,
				Command:   options.Command,
				Stdin:     true,
				Stdout:    true,
				Stderr:    true,
			}, scheme.ParameterCodec)

		exec, err := remotecommand.NewSPDYExecutor(h.config, "POST", req.URL())
		if err != nil {
			return false, err
		}

		var stdout, stderr bytes.Buffer
		stdin := bytes.NewReader(options.Stdin)
		err = exec.StreamWithContext(h.ctx, remotecommand.StreamOptions{
			Stdin:  stdin,
			Stdout: &stdout,
			Stderr: &stderr,
			Tty:    true,
		})
		if err != nil {
			return false, err
		}

		result = PodExecResult{
			Stdout: stdout.Bytes(),
			Stderr: stderr.Bytes(),
		}
		return true, nil
	})
	return &result, err
}
