package helpers

import (
	"context"
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// ServiceHelper implements functions for dealing with services
type ServiceHelper interface {
	// WaitServiceReady waits for the given service to have at least one endpoint available
	WaitServiceReady(ctx context.Context, service string, timeout time.Duration) error
	// WaitIngressReady waits for the given service to have a load balancer address assigned
	WaitIngressReady(ctx context.Context, ingress string, timeout time.Duration) error
	// GetTargets returns the list of pods that match the service selector criteria
	GetTargets(ctx context.Context, service string) ([]corev1.Pod, error)
}

// helpers struct holds the data required by the helpers
type serviceHelper struct {
	client    kubernetes.Interface
	namespace string
}

// NewServiceHelper returns a ServiceHelper
func NewServiceHelper(client kubernetes.Interface, namespace string) ServiceHelper {
	return &serviceHelper{
		client:    client,
		namespace: namespace,
	}
}

func (h *serviceHelper) WaitServiceReady(ctx context.Context, service string, timeout time.Duration) error {
	return utils.Retry(timeout, time.Second, func() (bool, error) {
		ep, err := h.client.CoreV1().Endpoints(h.namespace).Get(ctx, service, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, fmt.Errorf("failed to access service: %w", err)
		}

		for _, subset := range ep.Subsets {
			if len(subset.Addresses) > 0 {
				return true, nil
			}
		}

		return false, nil
	})
}

func (h *serviceHelper) WaitIngressReady(ctx context.Context, name string, timeout time.Duration) error {
	return utils.Retry(timeout, time.Second, func() (bool, error) {
		ingress, err := h.client.NetworkingV1().Ingresses(h.namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, fmt.Errorf("failed to access service: %w", err)
		}

		hasAddress := len(ingress.Status.LoadBalancer.Ingress) > 0

		return hasAddress, nil
	})
}

func (h *serviceHelper) GetTargets(ctx context.Context, name string) ([]corev1.Pod, error) {
	service, err := h.client.CoreV1().Services(h.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve target service %s: %w", service, err)
	}

	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(service.Spec.Selector).String(),
	}
	pods, err := h.client.CoreV1().Pods(h.namespace).List(
		ctx,
		listOptions,
	)

	return pods.Items, err
}
