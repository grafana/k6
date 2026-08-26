package kubernetes

import (
	"context"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

// FakeKubernetes is a fake implementation of the Kubernetes interface
type FakeKubernetes struct {
	client   *fake.Clientset
	ctx      context.Context
	executor *helpers.FakePodCommandExecutor
}

// NewFakeKubernetes returns a new fake implementation of Kubernetes from fake Clientset
func NewFakeKubernetes(clientset *fake.Clientset) (*FakeKubernetes, error) {
	return &FakeKubernetes{
		client:   clientset,
		ctx:      context.TODO(),
		executor: helpers.NewFakePodCommandExecutor(),
	}, nil
}

// PodHelper returns a PodHelper for the given namespace
func (f *FakeKubernetes) PodHelper(namespace string) helpers.PodHelper {
	return helpers.NewPodHelper(
		f.client,
		f.executor,
		namespace,
	)
}

// ServiceHelper returns a ServiceHelper for the given namespace
func (f *FakeKubernetes) ServiceHelper(namespace string) helpers.ServiceHelper {
	return helpers.NewServiceHelper(
		f.client,
		namespace,
	)
}

// Client return a kubernetes client
func (f *FakeKubernetes) Client() kubernetes.Interface {
	return f.client
}

// GetFakeProcessExecutor returns the FakeProcessExecutor used by the helpers to mock
// the execution of commands in a Pod
func (f *FakeKubernetes) GetFakeProcessExecutor() *helpers.FakePodCommandExecutor {
	return f.executor
}
