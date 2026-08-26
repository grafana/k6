// Package kubernetes implements helper functions for manipulating resources in a
// Kubernetes cluster.
package kubernetes

import (
	"errors"
	"fmt"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Kubernetes defines an interface that extends kubernetes interface[k8s.io/client-go/kubernetes.Interface]
// Adding helper functions for common tasks
type Kubernetes interface {
	// Client returns a Kubernetes client
	Client() kubernetes.Interface
	// ServiceHelper returns a helpers.ServiceHelper scoped for the given namespace
	ServiceHelper(namespace string) helpers.ServiceHelper
	// PodHelper returns a helpers.PodHelper scoped for the given namespace
	PodHelper(namespace string) helpers.PodHelper
}

// k8s Holds the reference to the helpers for interacting with kubernetes
type k8s struct {
	config *rest.Config
	kubernetes.Interface
}

// NewFromConfig returns a Kubernetes instance configured with the provided kubeconfig.
func NewFromConfig(config *rest.Config) (Kubernetes, error) {
	// As per the discussion in [1] client side rate limiting is no longer required.
	// Setting a large limit
	// [1] https://github.com/kubernetes/kubernetes/issues/111880
	config.QPS = 100
	config.Burst = 150

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	err = checkK8sVersion(config)
	if err != nil {
		return nil, err
	}

	return &k8s{
		config:    config,
		Interface: client,
	}, nil
}

// NewFromKubeconfig returns a Kubernetes instance configured with the kubeconfig pointed by the given path
func NewFromKubeconfig(kubeconfig string) (Kubernetes, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	return NewFromConfig(config)
}

// New returns a Kubernetes instance or an error when no config is eligible to be used.
// there are three ways of loading the kubernetes config, using the order as they are described below
// 1. in-cluster config, from serviceAccount token.
// 2. KUBECONFIG environment variable.
// 3. $HOME/.kube/config file.
func New() (Kubernetes, error) {
	k8sConfig, err := rest.InClusterConfig()
	if err == nil {
		return NewFromConfig(k8sConfig)
	}

	if !errors.Is(err, rest.ErrNotInCluster) {
		return nil, err
	}

	kubeConfigPath, getConfigErr := getConfigPath()
	if getConfigErr != nil {
		return nil, fmt.Errorf("error getting kubernetes config path: %w", getConfigErr)
	}

	return NewFromKubeconfig(kubeConfigPath)
}

func checkK8sVersion(config *rest.Config) error {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}

	version, err := discoveryClient.ServerVersion()
	if err != nil {
		return err
	}

	semver := fmt.Sprintf("v%s.%s", version.Major, version.Minor)
	// TODO: implement proper semver check
	if semver < "v1.23" {
		return fmt.Errorf("unsupported Kubernetes version. Expected >= v1.23 but actual is %s", semver)
	}
	return nil
}

// ServiceHelper returns a ServiceHelper for the given namespace
func (k *k8s) ServiceHelper(namespace string) helpers.ServiceHelper {
	return helpers.NewServiceHelper(
		k.Interface,
		namespace,
	)
}

// PodHelper returns a PodHelper for the given namespace
func (k *k8s) PodHelper(namespace string) helpers.PodHelper {
	executor := helpers.NewRestExecutor(k.CoreV1().RESTClient(), k.config)
	return helpers.NewPodHelper(
		k,
		executor,
		namespace,
	)
}

func (k *k8s) Client() kubernetes.Interface {
	return k.Interface
}
