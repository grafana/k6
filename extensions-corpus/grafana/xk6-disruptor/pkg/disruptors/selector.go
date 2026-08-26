package disruptors

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ErrSelectorNoPods is returned by NewPodDisruptor when the selector passed to it does not match any pod in the
// cluster.
var ErrSelectorNoPods = errors.New("no pods found matching selector")

// ErrServiceNoTargets is returned by NewServiceDisruptor when passed a service without any pod matching its selector.
var ErrServiceNoTargets = errors.New("service does not have any backing pods")

// PodSelector returns the target of a PodSelectorSpec
type PodSelector struct {
	helper helpers.PodHelper
	spec   PodSelectorSpec
}

// NewPodSelector creates a new PodSelector
func NewPodSelector(spec PodSelectorSpec, helper helpers.PodHelper) (*PodSelector, error) {
	// validate selector
	emptySelect := reflect.DeepEqual(spec.Select, PodAttributes{})
	emptyExclude := reflect.DeepEqual(spec.Exclude, PodAttributes{})
	if spec.Namespace == "" && emptySelect && emptyExclude {
		return nil, fmt.Errorf("namespace, select and exclude attributes in pod selector cannot all be empty")
	}

	return &PodSelector{
		spec:   spec,
		helper: helper,
	}, nil
}

// Targets returns the list of target pods
func (s *PodSelector) Targets(ctx context.Context) ([]corev1.Pod, error) {
	filter := helpers.PodFilter{
		Select:  s.spec.Select.Labels,
		Exclude: s.spec.Exclude.Labels,
	}

	targets, err := s.helper.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("finding pods matching '%s': %w", s.spec, ErrSelectorNoPods)
	}

	return targets, nil
}

// NamespaceOrDefault returns the configured namespace for this selector, and the name of the default namespace if it
// is not configured.
func (p PodSelectorSpec) NamespaceOrDefault() string {
	if p.Namespace != "" {
		return p.Namespace
	}

	return metav1.NamespaceDefault
}

// String returns a human-readable explanation of the pods matched by a PodSelector.
func (p PodSelectorSpec) String() string {
	var str string

	if len(p.Select.Labels) == 0 && len(p.Exclude.Labels) == 0 {
		str = "all pods"
	} else {
		str = "pods "
		str += p.groupLabels("including", p.Select.Labels)
		str += p.groupLabels("excluding", p.Exclude.Labels)
		str = strings.TrimSuffix(str, ", ")
	}

	str += fmt.Sprintf(" in ns %q", p.NamespaceOrDefault())

	return str
}

// groupLabels returns a group of labels as a string, giving that group a name. The returned string has the form of:
// `groupName(foo=bar, boo=baz), `, including the trailing space and comma.
// An empty group of labels produces an empty string.
func (PodSelectorSpec) groupLabels(groupName string, labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	group := groupName + "("
	for k, v := range labels {
		group += fmt.Sprintf("%s=%s, ", k, v)
	}
	group = strings.TrimSuffix(group, ", ")
	group += "), "

	return group
}

// ServicePodSelector returns the targets of a Service
type ServicePodSelector struct {
	service   string
	namespace string
	helper    helpers.ServiceHelper
}

// NewServicePodSelector returns a new ServicePodSelector
func NewServicePodSelector(
	service string,
	namespace string,
	helper helpers.ServiceHelper,
) (*ServicePodSelector, error) {
	return &ServicePodSelector{
		service:   service,
		namespace: namespace,
		helper:    helper,
	}, nil
}

// Targets returns the list of target pods
func (s *ServicePodSelector) Targets(ctx context.Context) ([]corev1.Pod, error) {
	targets, err := s.helper.GetTargets(ctx, s.service)
	if err != nil {
		return nil, err
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("finding pods matching%s/%s: %w", s.service, s.namespace, ErrServiceNoTargets)
	}

	return targets, nil
}
