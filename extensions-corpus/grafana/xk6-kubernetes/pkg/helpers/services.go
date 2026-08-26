package helpers

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-kubernetes/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

// ServiceHelper implements functions for dealing with services
type ServiceHelper interface {
	// WaitServiceReady waits for the given service to have at least one endpoint available
	// or the timeout (in seconds) expires. It returns a boolean indicating if the service is ready
	WaitServiceReady(service string, timeout int64) (bool, error)
	// GetExternalIP returns one external ip for the given service. If none is assigned after the timeout
	// expires, returns an empty address "".
	GetExternalIP(service string, timeout int64) (string, error)
}

func (h *helpers) WaitServiceReady(service string, timeout int64) (bool, error) {
	return utils.Retry(time.Duration(timeout)*time.Second, time.Second, func() (bool, error) {
		ep := &corev1.Endpoints{}
		err := h.client.Structured().Get("Endpoints", service, h.namespace, ep)
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

func (h *helpers) GetExternalIP(service string, timeout int64) (string, error) {
	addr := ""
	_, err := utils.Retry(time.Duration(timeout)*time.Second, time.Second, func() (bool, error) {
		svc := &corev1.Service{}
		err := h.client.Structured().Get("Service", service, h.namespace, svc)
		if err != nil {
			return false, fmt.Errorf("failed to access service: %w", err)
		}

		if len(svc.Status.LoadBalancer.Ingress) > 0 {
			addr = svc.Status.LoadBalancer.Ingress[0].IP
			return true, nil
		}

		return false, nil
	})

	return addr, err
}
