//go:build integration
// +build integration

package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func Test_DefaultConfig(t *testing.T) {
	// create cluster with default configuration
	config, err := NewConfig(
		"default-cluster",
		Options{
			Wait: time.Second * 60,
		},
	)
	if err != nil {
		t.Errorf("failed creating cluster configuration: %v", err)
		return
	}

	cluster, err := config.Create()
	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
		return
	}

	// delete cluster
	cluster.Delete()
}

func Test_UseEtcdRamDisk(t *testing.T) {
	// create cluster with default configuration
	config, err := NewConfig(
		"etcd-ramdisk-cluster",
		Options{
			Wait:           time.Second * 60,
			UseEtcdRAMDisk: true,
		},
	)
	if err != nil {
		t.Errorf("failed creating cluster configuration: %v", err)
		return
	}

	cluster, err := config.Create()
	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
		return
	}

	// delete cluster
	cluster.Delete()
}

func getKubernetesClient(kubeconfig string) (kubernetes.Interface, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func Test_PreloadImages(t *testing.T) {
	// create cluster with preloaded images
	config, err := NewConfig(
		"cluster-with-images",
		Options{
			Wait:   time.Second * 60,
			Images: []string{"busybox"},
		},
	)
	if err != nil {
		t.Errorf("failed to create cluster config: %v", err)
		return
	}

	cluster, err := config.Create()
	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
		return
	}

	defer cluster.Delete()

	kubeconfig := cluster.Kubeconfig()

	k8s, err := getKubernetesClient(kubeconfig)
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}

	// create pod with busybox preventing pulling image if not already present in the node
	busybox := builders.NewPodBuilder("busybox").
		WithContainer(
			builders.NewContainerBuilder("busybox").
				WithImage("busybox").
				WithPullPolicy(corev1.PullNever).
				Build(),
		).
		Build()

	_, err = k8s.CoreV1().Pods("default").Create(context.TODO(), &busybox, metav1.CreateOptions{})
	if err != nil {
		t.Errorf("failed to create pod: %v", err)
		return
	}
	// FIXME: using hardcoded waits is flaky
	time.Sleep(time.Second * 5)

	created, err := k8s.CoreV1().Pods("default").Get(context.TODO(), "busybox", metav1.GetOptions{})
	if err != nil {
		t.Errorf("failed to get pod: %v", err)
		return
	}

	waiting := created.Status.ContainerStatuses[0].State.Waiting
	if waiting != nil && (waiting.Reason == "ErrImageNeverPull") {
		t.Errorf("pod is waiting for image")
		return
	}
}

func Test_KubernetesVersion(t *testing.T) {
	// create cluster with default configuration
	config, err := NewConfig(
		"k8s-1-24-cluster",
		Options{
			Version: "v1.24.0",
			Wait:    time.Second * 60,
		},
	)
	if err != nil {
		t.Errorf("failed creating cluster configuration: %v", err)
		return
	}

	cluster, err := config.Create()
	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
		return
	}

	// delete cluster
	cluster.Delete()
}

func Test_InvalidKubernetesVersion(t *testing.T) {
	// create cluster with default configuration
	config, err := NewConfig(
		"invalid-k8s-version-cluster",
		Options{
			Version: "v0.0.0",
			Wait:    time.Second * 60,
		},
	)
	if err != nil {
		t.Errorf("failed creating cluster configuration: %v", err)
		return
	}

	cluster, err := config.Create()
	if err == nil {
		t.Errorf("Should have failed creating cluster")
		cluster.Delete()
		return
	}
}

// FIXME: this is a very basic test. Check for error conditions and ensure
// returned cluster is functional.
func Test_GetCluster(t *testing.T) {
	// create cluster with  configuration
	config, err := NewConfig(
		"preexisting-cluster",
		Options{
			Wait: time.Second * 60,
		},
	)
	if err != nil {
		t.Errorf("failed creating cluster configuration: %v", err)
		return
	}

	c, err := config.Create()
	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
		return
	}

	cluster, err := GetCluster(c.Name(), c.Kubeconfig())
	if err != nil {
		t.Errorf("failed to get cluster: %v", err)
		return
	}

	// delete cluster
	cluster.Delete()
	if err != nil {
		t.Errorf("failed to delete cluster: %v", err)
		return
	}
}

func Test_DeleteCluster(t *testing.T) {
	// create cluster with  configuration
	config, err := NewConfig(
		"for-delete-cluster",
		Options{
			Wait: time.Second * 30,
		},
	)
	if err != nil {
		t.Errorf("failed creating cluster configuration: %v", err)
		return
	}

	_, err = config.Create()
	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
		return
	}

	testCases := []struct {
		test        string
		name        string
		quiet       bool
		expectError bool
	}{
		{
			test:        "delete existing cluster",
			name:        "for-delete-cluster",
			quiet:       false,
			expectError: false,
		},
		{
			test:        "delete non-existing cluster",
			name:        "non-existing-cluster",
			quiet:       false,
			expectError: true,
		},
		{
			test:        "delete non-existing cluster with quiet option",
			name:        "non-existing-cluster",
			quiet:       true,
			expectError: false,
		},
	}
	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			err = DeleteCluster(tc.name, tc.quiet)
			if err != nil && !tc.expectError {
				t.Fatalf("failed deleting cluster: %v", err)
			}

			if err == nil && tc.expectError {
				t.Fatalf("should had failed deleting cluster")
			}
		})
	}
}
