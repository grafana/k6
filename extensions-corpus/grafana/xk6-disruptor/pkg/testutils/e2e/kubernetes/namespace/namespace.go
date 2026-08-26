// Package namespace implements helper functions for manipulating kubernetes namespaces
package namespace

import (
	"context"
	"fmt"
	"testing"

	"github.com/grafana/xk6-disruptor/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// TestNamespaceOption allows modifying an TestNamespaceConfig
type TestNamespaceOption func(TestNamespaceConfig) (TestNamespaceConfig, error)

// TestNamespaceConfig defines the options for creating a test mamespace
type TestNamespaceConfig struct {
	keepOnFail bool
	random     bool
	name       string
}

// DefaultNamespaceConfig defines the default options for creating a test namespace
func DefaultNamespaceConfig() TestNamespaceConfig {
	return TestNamespaceConfig{
		keepOnFail: true,
		random:     true,
		name:       "testns-",
	}
}

// WithPrefix sets the name of the namespace to a random name with the given prefix
func WithPrefix(prefix string) TestNamespaceOption {
	return func(c TestNamespaceConfig) (TestNamespaceConfig, error) {
		if prefix == "" {
			return c, fmt.Errorf("prefix cannot be empty")
		}

		c.name = prefix
		c.random = true
		return c, nil
	}
}

// WithName sets the name of the namespace
func WithName(name string) TestNamespaceOption {
	return func(c TestNamespaceConfig) (TestNamespaceConfig, error) {
		if name == "" {
			return c, fmt.Errorf("name cannot be empty")
		}

		c.name = name
		c.random = false
		return c, nil
	}
}

// WithKeepOnFail indicates if the namespace must be kept in case the test fails
func WithKeepOnFail(keepOnFail bool) TestNamespaceOption {
	return func(c TestNamespaceConfig) (TestNamespaceConfig, error) {
		c.keepOnFail = keepOnFail
		return c, nil
	}
}

func mergeEnvVariables(config TestNamespaceConfig) TestNamespaceConfig {
	config.keepOnFail = utils.GetBooleanEnvVar("E2E_KEEPONFAIL", config.keepOnFail)
	return config
}

// CreateTestNamespace creates a namespace for testing
func CreateTestNamespace(
	ctx context.Context,
	t *testing.T,
	k8s kubernetes.Interface,
	options ...TestNamespaceOption,
) (string, error) {
	var err error

	config := DefaultNamespaceConfig()
	for _, option := range options {
		config, err = option(config)
		if err != nil {
			return "", err
		}
	}

	config = mergeEnvVariables(config)

	ns := &corev1.Namespace{}
	if config.random {
		ns.ObjectMeta = metav1.ObjectMeta{GenerateName: config.name}
	} else {
		ns.ObjectMeta = metav1.ObjectMeta{Name: config.name}
	}

	ns, err = k8s.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create test namespace %q: %w", config.name, err)
	}

	t.Cleanup(func() {
		if t.Failed() && config.keepOnFail {
			return
		}

		_ = k8s.CoreV1().Namespaces().Delete(ctx, config.name, metav1.DeleteOptions{})
	})

	return ns.GetName(), nil
}
