// Package helpers offers functions to simplify dealing with kubernetes resources.
package helpers

import (
	"context"

	k8s "k8s.io/client-go/kubernetes"

	"github.com/grafana/xk6-kubernetes/pkg/resources"
	"k8s.io/client-go/rest"
)

// Helpers offers Helper functions grouped by the objects they handle
type Helpers interface {
	JobHelper
	PodHelper
	ServiceHelper
}

// helpers struct holds the data required by the helpers
type helpers struct {
	client    *resources.Client
	clientset k8s.Interface
	config    *rest.Config
	ctx       context.Context
	namespace string
}

// NewHelper creates a set of helper functions on the specified namespace
func NewHelper(
	ctx context.Context,
	clientset k8s.Interface,
	client *resources.Client,
	config *rest.Config,
	namespace string,
) Helpers {
	return &helpers{
		client:    client,
		clientset: clientset,
		config:    config,
		ctx:       ctx,
		namespace: namespace,
	}
}
