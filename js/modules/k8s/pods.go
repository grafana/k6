package k8s

import (
	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Pods contains all available methods for interacting with k8s pods
type Pods struct {
	client *kubernetes.Clientset
}

// NewPods creates a new instance of the Pods struct
func NewPods(client *kubernetes.Clientset) *Pods {
	return &Pods{
		client,
	}
}

// List all pods in a namespace
func (pods *Pods) List(namespace string) ([]string, error) {
	podList, err := pods.client.CoreV1().Pods(namespace).List(v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	alivePods := make([]string, 0)
	for _, pod := range podList.Items {
		if pod.DeletionTimestamp != nil {
			continue
		}
		alivePods = append(alivePods, pod.Name)
	}

	return alivePods, nil
}

// Kill a pod with a specific name in a specific namespace
func (pods *Pods) Kill(namespace string, podName string) error {
	podsInNamespace := pods.client.CoreV1().Pods(namespace)
	err := podsInNamespace.Delete(podName, &v1.DeleteOptions{})

	return err
}

// Status of a pod with a specific name in a specific namespace
func (pods *Pods) Status(namespace string, podName string) (coreV1.PodStatus, error) {
	pod, err := pods.client.CoreV1().Pods(namespace).Get(podName, v1.GetOptions{})

	return pod.Status, err
}
