//go:build integration
// +build integration

package kubernetes

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/fixtures"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/k3s"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/tools/clientcmd"
)

func createRandomTestNamespace(k8s Kubernetes) (string, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
		},
	}

	ns, err := k8s.Client().CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create test namespace %w", err)
	}

	return ns.Name, nil
}

func Test_Kubernetes(t *testing.T) {
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

	k8s, err := NewFromConfig(restcfg)
	if err != nil {
		t.Fatalf("error creating kubernetes client: %v", err)
	}

	// Test Wait Pod Running
	t.Run("Wait Pod is Running", func(t *testing.T) {
		t.Parallel()

		namespace, err := createRandomTestNamespace(k8s)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = k8s.Client().CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
		})

		// Deploy nginx
		nginx := fixtures.BuildNginxPod()
		_, err = k8s.Client().CoreV1().Pods(namespace).Create(
			context.TODO(),
			&nginx,
			metav1.CreateOptions{},
		)
		if err != nil {
			t.Fatalf("failed to create pod: %v", err)
		}

		// wait for the pod to be ready for accepting requests
		running, err := k8s.PodHelper(namespace).WaitPodRunning(context.TODO(), "nginx", time.Second*20)
		if err != nil {
			t.Fatalf("error waiting for pod %v", err)
		}
		if !running {
			t.Fatalf("timeout expired waiting for pod ready")
		}
	})

	// Test Wait Service Ready helper
	t.Run("Wait Service Ready", func(t *testing.T) {
		t.Parallel()

		namespace, err := createRandomTestNamespace(k8s)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = k8s.Client().CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
		})

		// Deploy nginx and expose it as a service. Intentionally not using e2e fixures
		// because these functions rely on WaitPodRunning and WaitServiceReady which we
		// are testing here.
		nginxPod := fixtures.BuildNginxPod()
		_, err = k8s.Client().CoreV1().Pods(namespace).Create(
			context.TODO(),
			&nginxPod,
			metav1.CreateOptions{},
		)
		if err != nil {
			t.Fatalf("failed to create pod: %v", err)
		}

		nginxSvc := fixtures.BuildNginxService()
		_, err = k8s.Client().CoreV1().Services(namespace).Create(
			context.TODO(),
			&nginxSvc,
			metav1.CreateOptions{},
		)
		if err != nil {
			t.Fatalf("failed to create nginx service: %v", err)
		}

		// wait for the service to be ready for accepting requests
		err = k8s.ServiceHelper(namespace).WaitServiceReady(context.TODO(), "nginx", time.Second*20)
		if err != nil {
			t.Fatalf("error waiting for service nginx: %v", err)
		}
	})

	t.Run("Exec Command", func(t *testing.T) {
		t.Parallel()

		namespace, err := createRandomTestNamespace(k8s)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = k8s.Client().CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
		})

		busybox := fixtures.BuildBusyBoxPod()
		_, err = k8s.Client().CoreV1().Pods(namespace).Create(
			context.TODO(),
			&busybox,
			metav1.CreateOptions{},
		)
		if err != nil {
			t.Fatalf("failed to create pod: %v", err)
		}

		// wait for the pod to be ready
		running, err := k8s.PodHelper(namespace).WaitPodRunning(context.TODO(), "busybox", time.Second*20)
		if err != nil {
			t.Fatalf("error waiting for pod %v", err)
		}
		if !running {
			t.Fatalf("timeout expired waiting for pod ready")
		}

		greetings := "hello world"
		cmd := []string{"echo", "-n", greetings}
		stdout, _, err := k8s.PodHelper(namespace).Exec(
			context.TODO(),
			"busybox",
			"busybox",
			cmd,
			nil,
		)
		if err != nil {
			t.Fatalf("error executing command in pod: %v", err)
		}

		if string(stdout) != greetings {
			t.Fatalf("stdout does not match expected result:\nexpected: %q\nactual%q\n", greetings, string(stdout))
		}
	})

	t.Run("Attach Ephemeral Container", func(t *testing.T) {
		t.Parallel()

		namespace, err := createRandomTestNamespace(k8s)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = k8s.Client().CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
		})

		paused := fixtures.BuildPausedPod()
		_, err = k8s.Client().CoreV1().Pods(namespace).Create(
			context.TODO(),
			&paused,
			metav1.CreateOptions{},
		)
		if err != nil {
			t.Fatalf("failed to create pod: %v", err)
		}

		ephemeral := corev1.EphemeralContainer{
			EphemeralContainerCommon: corev1.EphemeralContainerCommon{
				Name:    "ephemeral",
				Image:   "busybox",
				Command: []string{"sleep", "300"},
				TTY:     true,
				Stdin:   true,
			},
		}

		err = k8s.PodHelper(namespace).AttachEphemeralContainer(
			context.TODO(),
			"paused",
			ephemeral,
			helpers.AttachOptions{
				Timeout: 15 * time.Second,
			},
		)
		if err != nil {
			t.Fatalf("error attaching ephemeral container to pod: %v", err)
		}

		stdout, _, err := k8s.PodHelper(namespace).Exec(
			context.TODO(),
			"paused",
			"ephemeral",
			[]string{"echo", "-n", "hello", "world"},
			nil,
		)
		if err != nil {
			t.Fatalf("error executing command in pod: %v", err)
		}

		greetings := "hello world"
		if string(stdout) != "hello world" {
			t.Fatalf("stdout does not match expected result:\nexpected: %q\nactual%q\n", greetings, string(stdout))
		}
	})
}

func Test_UnsupportedKubernetesVersion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	container, err := k3s.RunContainer(ctx, testcontainers.WithImage("docker.io/rancher/k3s:v1.22.17-k3s1"))
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

	_, err = NewFromConfig(restcfg)
	if err == nil {
		t.Errorf("should had failed creating kubernetes client")
		return
	}
}
