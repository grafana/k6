// Package api implements helper functions for manipulating resources in a
// Kubernetes cluster.
package api

import (
	"context"

	k8s "k8s.io/client-go/kubernetes"

	"github.com/grafana/xk6-kubernetes/pkg/helpers"
	"github.com/grafana/xk6-kubernetes/pkg/resources"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

// Kubernetes defines an interface that extends kubernetes interface[k8s.io/client-go/kubernetes.Interface] adding
// generic functions that operate on any kind of object
type Kubernetes interface {
	resources.UnstructuredOperations
	// Helpers returns helpers for the given namespace. If none is specified, "default" is used
	Helpers(namespace string) helpers.Helpers
}

// KubernetesConfig defines the configuration for creating a Kubernetes instance
type KubernetesConfig struct {
	// Context for executing kubernetes operations
	Context context.Context
	// kubernetes rest config
	Config *rest.Config
	// Clientset provides access to various API-specific clients
	Clientset k8s.Interface
	// Client is a pre-configured dynamic client. If provided, the rest config is not used
	Client dynamic.Interface
	// Mapper is a pre-configured RESTMapper. If provided, the rest config is not used
	Mapper meta.RESTMapper
}

// kubernetes holds references to implementation of the Kubernetes interface
type kubernetes struct {
	ctx       context.Context
	Clientset k8s.Interface
	*resources.Client
	Config *rest.Config
	*restmapper.DeferredDiscoveryRESTMapper
}

// NewFromConfig returns a Kubernetes instance
func NewFromConfig(c KubernetesConfig) (Kubernetes, error) {
	var (
		err             error
		discoveryClient *discovery.DiscoveryClient
	)

	ctx := c.Context
	if ctx == nil {
		ctx = context.TODO()
	}

	var client *resources.Client
	if c.Client != nil {
		client = resources.NewFromClient(ctx, c.Client).WithMapper(c.Mapper)
	} else {
		client, err = resources.NewFromConfig(ctx, c.Config)
		if err != nil {
			return nil, err
		}
	}

	if c.Mapper == nil {
		discoveryClient, err = discovery.NewDiscoveryClientForConfig(c.Config)
		mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))
		if err != nil {
			return nil, err
		}
		client.WithMapper(mapper)
	}

	return &kubernetes{
		ctx:       ctx,
		Clientset: c.Clientset,
		Client:    client,
		Config:    c.Config,
	}, nil
}

func (k *kubernetes) Helpers(namespace string) helpers.Helpers {
	if namespace == "" {
		namespace = "default"
	}
	return helpers.NewHelper(
		k.ctx,
		k.Clientset,
		k.Client,
		k.Config,
		namespace,
	)
}
