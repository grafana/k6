// Package disruptors implements an API for disrupting targets
package disruptors

import (
	"context"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"
	"github.com/grafana/xk6-disruptor/pkg/types/intstr"
	"github.com/grafana/xk6-disruptor/pkg/utils"
)

// DefaultTargetPort defines the default value for a target HTTP
var DefaultTargetPort = intstr.FromInt32(80) //nolint:gochecknoglobals

// PodDisruptor defines the types of faults that can be injected in a Pod
type PodDisruptor interface {
	Disruptor
	ProtocolFaultInjector
	PodFaultInjector
	NetworkFaultInjector
}

// PodDisruptorOptions defines options that controls the PodDisruptor's behavior
type PodDisruptorOptions struct {
	// timeout when waiting agent to be injected in seconds. A zero value forces default.
	// A Negative value forces no waiting.
	InjectTimeout time.Duration `js:"injectTimeout"`
}

// podDisruptor is an instance of a PodDisruptor that uses a PodController to interact with target pods
type podDisruptor struct {
	helper   helpers.PodHelper
	selector *PodSelector
	options  PodDisruptorOptions
}

// PodSelectorSpec defines the criteria for selecting a pod for disruption
type PodSelectorSpec struct {
	Namespace string
	// Select Pods that match these PodAttributes
	Select PodAttributes
	// Select Pods that match these PodAttributes
	Exclude PodAttributes
}

// PodAttributes defines the attributes a Pod must match for being selected/excluded
type PodAttributes struct {
	Labels map[string]string
}

// NewPodDisruptor creates a new instance of a PodDisruptor that acts on the pods
// that match the given PodSelector
func NewPodDisruptor(
	_ context.Context,
	k8s kubernetes.Kubernetes,
	spec PodSelectorSpec,
	options PodDisruptorOptions,
) (PodDisruptor, error) {
	// ensure selector and controller use default namespace if none specified
	namespace := spec.NamespaceOrDefault()

	helper := k8s.PodHelper(namespace)

	selector, err := NewPodSelector(spec, helper)
	if err != nil {
		return nil, err
	}

	return &podDisruptor{
		helper:   helper,
		options:  options,
		selector: selector,
	}, nil
}

func (d *podDisruptor) Targets(ctx context.Context) ([]string, error) {
	targets, err := d.selector.Targets(ctx)
	if err != nil {
		return nil, err
	}

	return utils.PodNames(targets), nil
}

// InjectHTTPFaults injects faults in the http requests sent to the disruptor's targets
func (d *podDisruptor) InjectHTTPFaults(
	ctx context.Context,
	fault HTTPFault,
	duration time.Duration,
	options HTTPDisruptionOptions,
) error {
	// Handle default port mapping
	// TODO: make port mandatory instead of using a default
	if fault.Port.IsNull() || fault.Port.IsZero() {
		fault.Port = DefaultTargetPort
	}

	command := PodHTTPFaultCommand{
		fault:    fault,
		duration: duration,
		options:  options,
	}

	visitor := NewPodAgentVisitor(
		d.helper,
		PodAgentVisitorOptions{Timeout: d.options.InjectTimeout},
		command,
	)

	targets, err := d.selector.Targets(ctx)
	if err != nil {
		return err
	}

	controller := NewPodController(targets)

	return controller.Visit(ctx, visitor)
}

// InjectGrpcFaults injects faults in the grpc requests sent to the disruptor's targets
func (d *podDisruptor) InjectGrpcFaults(
	ctx context.Context,
	fault GrpcFault,
	duration time.Duration,
	options GrpcDisruptionOptions,
) error {
	command := PodGrpcFaultCommand{
		fault:    fault,
		duration: duration,
		options:  options,
	}

	visitor := NewPodAgentVisitor(
		d.helper,
		PodAgentVisitorOptions{Timeout: d.options.InjectTimeout},
		command,
	)

	targets, err := d.selector.Targets(ctx)
	if err != nil {
		return err
	}

	controller := NewPodController(targets)

	return controller.Visit(ctx, visitor)
}

// TerminatePods terminates a subset of the target pods of the disruptor
func (d *podDisruptor) TerminatePods(
	ctx context.Context,
	fault PodTerminationFault,
) ([]string, error) {
	targets, err := d.selector.Targets(ctx)
	if err != nil {
		return nil, err
	}

	targets, err = utils.Sample(targets, fault.Count)
	if err != nil {
		return nil, err
	}

	controller := NewPodController(targets)

	visitor := PodTerminationVisitor{helper: d.helper, timeout: fault.Timeout}

	return utils.PodNames(targets), controller.Visit(ctx, visitor)
}

// InjectNetworkFaults injects network faults in the target pods
func (d *podDisruptor) InjectNetworkFaults(
	ctx context.Context,
	fault NetworkFault,
	duration time.Duration,
) error {
	command := PodNetworkFaultCommand{
		fault:    fault,
		duration: duration,
	}

	visitor := NewPodAgentVisitor(
		d.helper,
		PodAgentVisitorOptions{Timeout: d.options.InjectTimeout},
		command,
	)

	targets, err := d.selector.Targets(ctx)
	if err != nil {
		return err
	}

	controller := NewPodController(targets)

	return controller.Visit(ctx, visitor)
}
