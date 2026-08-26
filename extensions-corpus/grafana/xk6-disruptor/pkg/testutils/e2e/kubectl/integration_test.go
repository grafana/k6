//go:build integration
// +build integration

package kubectl

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/deploy"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/kubernetes/namespace"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"

	"github.com/testcontainers/testcontainers-go/modules/k3s"

	"k8s.io/client-go/tools/clientcmd"
)

func Test_Kubectl(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	container, err := k3s.RunContainer(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Clean up the container after the test is complete
	t.Cleanup(func() {
		if err = container.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	kubeConfigYaml, err := container.GetKubeConfig(ctx)
	if err != nil {
		t.Fatalf("failed to get kube-config : %s", err)
	}

	restcfg, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigYaml)
	if err != nil {
		t.Fatalf("failed to create rest client for kubernetes : %s", err)
	}

	k8s, err := kubernetes.NewFromConfig(restcfg)
	if err != nil {
		t.Fatalf("error creating kubernetes client: %v", err)
	}

	t.Run("Test port forwarding", func(t *testing.T) {
		namespace, err := namespace.CreateTestNamespace(context.TODO(), t, k8s.Client())
		if err != nil {
			t.Errorf("failed to create test namespace: %v", err)
			return
		}

		// Deploy nginx
		nginx := builders.NewPodBuilder("nginx").
			WithContainer(
				builders.NewContainerBuilder("nginx").
					WithImage("nginx").
					WithPort("http", 80).
					Build(),
			).
			Build()

		err = deploy.RunPod(k8s, namespace, nginx, 20*time.Second)
		if err != nil {
			t.Errorf("failed to create test pod: %v", err)
			return
		}

		client, err := NewForConfig(context.TODO(), restcfg)
		if err != nil {
			t.Errorf("failed to create kubectl client: %v", err)
			return
		}

		ctx, stopper := context.WithCancel(context.TODO())
		// ensure por forwarder is cancelled
		defer stopper()

		port, err := client.ForwardPodPort(ctx, namespace, nginx.GetName(), 80)
		if err != nil {
			t.Errorf("failed to forward local port: %v", err)
			return
		}

		url := fmt.Sprintf("http://localhost:%d", port)
		request, err := http.NewRequest("GET", url, bytes.NewReader([]byte{}))
		if err != nil {
			t.Errorf("failed to create request: %v", err)
			return
		}

		resp, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Errorf("failed make request: %v", err)
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status code %d but %d received", http.StatusOK, resp.StatusCode)
			return
		}
	})
}
