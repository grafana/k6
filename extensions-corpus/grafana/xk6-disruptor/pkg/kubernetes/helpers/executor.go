package helpers

import (
	"bytes"
	"context"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// PodCommandExecutor defines a method for executing commands in a target Pod
type PodCommandExecutor interface {
	// Exec executes a non-interactive command described in options and returns the stdout and stderr outputs
	Exec(
		ctx context.Context,
		pod string,
		namespace string,
		container string,
		command []string,
		stdin []byte,
	) ([]byte, []byte, error)
}

type restExecutor struct {
	client rest.Interface
	config *rest.Config
}

// NewRestExecutor returns a PodCommandExecutor that executes command using rest client with the
// given rest configuration
func NewRestExecutor(client rest.Interface, config *rest.Config) PodCommandExecutor {
	return &restExecutor{
		client: client,
		config: config,
	}
}

func (h *restExecutor) Exec(
	ctx context.Context,
	pod string,
	namespace string,
	container string,
	command []string,
	stdin []byte,
) ([]byte, []byte, error) {
	req := h.client.
		Post().
		Namespace(namespace).
		Resource("pods").
		Name(pod).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(h.config, "POST", req.URL())
	if err != nil {
		return nil, nil, err
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(
		ctx,
		remotecommand.StreamOptions{
			Stdin:  bytes.NewReader(stdin),
			Stdout: &stdout,
			Stderr: &stderr,
			Tty:    false,
		},
	)

	return stdout.Bytes(), stderr.Bytes(), err
}
