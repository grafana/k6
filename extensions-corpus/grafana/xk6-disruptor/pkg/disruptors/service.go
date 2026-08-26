package disruptors

import (
	"context"
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"
	"github.com/grafana/xk6-disruptor/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceDisruptor defines operations for injecting faults in services
type ServiceDisruptor interface {
	Disruptor
	ProtocolFaultInjector
	PodFaultInjector
}

// ServiceDisruptorOptions defines options that controls the behavior of the ServiceDisruptor
type ServiceDisruptorOptions struct {
	// timeout when waiting agent to be injected (default 30s). A zero value forces default.
	// A Negative value forces no waiting.
	InjectTimeout time.Duration `js:"injectTimeout"`
}

// serviceDisruptor is an instance of a ServiceDisruptor
type serviceDisruptor struct {
	service  corev1.Service
	helper   helpers.PodHelper
	selector *ServicePodSelector
	options  ServiceDisruptorOptions
}

// NewServiceDisruptor creates a new instance of a ServiceDisruptor that targets the given service
func NewServiceDisruptor(
	ctx context.Context,
	k8s kubernetes.Kubernetes,
	service string,
	namespace string,
	options ServiceDisruptorOptions,
) (ServiceDisruptor, error) {
	if service == "" {
		return nil, fmt.Errorf("must specify a service name")
	}

	if namespace == "" {
		return nil, fmt.Errorf("must specify a namespace")
	}

	svc, err := k8s.Client().CoreV1().Services(namespace).Get(ctx, service, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	selector, err := NewServicePodSelector(service, namespace, k8s.ServiceHelper(namespace))
	if err != nil {
		return nil, err
	}

	return &serviceDisruptor{
		service:  *svc,
		helper:   k8s.PodHelper(namespace),
		selector: selector,
		options:  options,
	}, nil
}

func (d *serviceDisruptor) InjectHTTPFaults(
	ctx context.Context,
	fault HTTPFault,
	duration time.Duration,
	options HTTPDisruptionOptions,
) error {
	// Map service port to a target pod port
	port, err := utils.GetTargetPort(d.service, fault.Port)
	if err != nil {
		return err
	}
	podFault := fault
	podFault.Port = port

	command := PodHTTPFaultCommand{
		fault:    podFault,
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

func (d *serviceDisruptor) InjectGrpcFaults(
	ctx context.Context,
	fault GrpcFault,
	duration time.Duration,
	options GrpcDisruptionOptions,
) error {
	// Map service port to a target pod port
	port, err := utils.GetTargetPort(d.service, fault.Port)
	if err != nil {
		return err
	}
	podFault := fault
	podFault.Port = port

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

func (d *serviceDisruptor) Targets(ctx context.Context) ([]string, error) {
	targets, err := d.selector.Targets(ctx)
	if err != nil {
		return nil, err
	}

	return utils.PodNames(targets), nil
}

// TerminatePods terminates a subset of the target pods of the disruptor
func (d *serviceDisruptor) TerminatePods(
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
