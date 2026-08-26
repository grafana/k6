package disruptors

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/internal/version"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"

	corev1 "k8s.io/api/core/v1"
)

// PodController uses a PodVisitor to perform a certain action (Visit) on a list of pods.
// The PodVisitor is responsible for executing the action in one target pod, while the PorController
// is responsible for coordinating the action of the PodVisitor on multiple target pods
type PodController struct {
	targets []corev1.Pod
}

// NewPodController creates a new controller for a collection of pods
func NewPodController(targets []corev1.Pod) *PodController {
	return &PodController{
		targets: targets,
	}
}

// Visit allows executing a different command on each target returned by a visiting function
func (c *PodController) Visit(ctx context.Context, visitor PodVisitor) error {
	// if there are no targets, nothing to do
	if len(c.targets) == 0 {
		return nil
	}

	// create context for the visit, that can be cancelled in case of error
	visitCtx, cancelVisit := context.WithCancel(ctx)
	defer cancelVisit()

	// make space to prevent blocking go routines
	doneCh := make(chan error, len(c.targets))

	for _, pod := range c.targets {
		go func(pod corev1.Pod) {
			doneCh <- visitor.Visit(visitCtx, pod)
		}(pod)
	}

	pending := len(c.targets)
	for {
		select {
		case e := <-doneCh:
			if e != nil {
				return e
			}
			pending--
			if pending == 0 {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// VisitCommands contains the commands to be executed when visiting a pod
type VisitCommands struct {
	Exec    []string
	Cleanup []string
}

// PodVisitor is the interface implemented by objects that perform actions on a Pod
type PodVisitor interface {
	Visit(context.Context, corev1.Pod) error
}

// PodVisitorFunc defines a function that implements the PodVisitor interface
// This allows using anonymous functions as pod visitor:
type PodVisitorFunc func(context.Context, corev1.Pod) error

// Visit implements PodVisitor interface's Visit function
func (f PodVisitorFunc) Visit(ctx context.Context, pod corev1.Pod) error {
	return f(ctx, pod)
}

// PodAgentVisitor implements PodVisitor, performing actions in a Pod by means of running a PodVisitCommand on the pod.
type PodAgentVisitor struct {
	helper  helpers.PodHelper
	options PodAgentVisitorOptions
	command PodVisitCommand
}

// NewPodAgentVisitor creates a new pod visitor
func NewPodAgentVisitor(
	helper helpers.PodHelper,
	options PodAgentVisitorOptions,
	command PodVisitCommand,
) *PodAgentVisitor {
	// FIXME: handling timeout < 0  is required only to allow tests to skip waiting for the agent injection
	if options.Timeout == 0 {
		options.Timeout = 30 * time.Second
	}
	if options.Timeout < 0 {
		options.Timeout = 0
	}

	return &PodAgentVisitor{
		helper:  helper,
		options: options,
		command: command,
	}
}

// injectDisruptorAgent injects the Disruptor agent in the target pods
func (c *PodAgentVisitor) injectDisruptorAgent(ctx context.Context, pod corev1.Pod) error {
	var (
		rootUser     = int64(0)
		rootGroup    = int64(0)
		runAsNonRoot = false
	)

	agentContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            "xk6-agent",
			Image:           version.AgentImage(),
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"NET_ADMIN"},
				},
				RunAsUser:    &rootUser,
				RunAsGroup:   &rootGroup,
				RunAsNonRoot: &runAsNonRoot,
			},
			TTY:   true,
			Stdin: true,
		},
	}

	return c.helper.AttachEphemeralContainer(
		ctx,
		pod.Name,
		agentContainer,
		helpers.AttachOptions{
			Timeout:        c.options.Timeout,
			IgnoreIfExists: true,
		},
	)
}

// Visit allows executing a different command on each target returned by a visiting function
func (c *PodAgentVisitor) Visit(ctx context.Context, pod corev1.Pod) error {
	err := c.injectDisruptorAgent(ctx, pod)
	if err != nil {
		return fmt.Errorf("injecting agent in the pod %q: %w", pod.Name, err)
	}

	// get the command to execute in the target
	commands, err := c.command.Commands(pod)
	if err != nil {
		return fmt.Errorf("unable to get command for pod %q: %w", pod.Name, err)
	}

	_, stderr, err := c.helper.Exec(ctx, pod.Name, "xk6-agent", commands.Exec, []byte{})

	if err != nil && commands.Cleanup != nil {
		// we ignore errors because we are reporting the reason of the exec failure
		// we use a fresh context because the context used in exec may have been cancelled or expired
		//nolint:contextcheck
		_, _, _ = c.helper.Exec(context.TODO(), pod.Name, "xk6-agent", commands.Cleanup, []byte{})
	}

	// if the context is cancelled, don't report error (we assume the caller is reporting this error)
	if err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("failed command execution for pod %q: %w \n%s", pod.Name, err, string(stderr))
	}

	return nil
}

// PodAgentVisitorOptions defines the options for the PodVisitor
type PodAgentVisitorOptions struct {
	// Defines the timeout for injecting the agent
	Timeout time.Duration
}

// PodVisitCommand is a command that can be run on a given pod.
// Implementations build the VisitCommands according to properties of the pod where it is going to run
type PodVisitCommand interface {
	// Commands defines the command to be executed, and optionally a cleanup command
	Commands(corev1.Pod) (VisitCommands, error)
}
