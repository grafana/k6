package k8s

import (
	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Pods struct {
	client *kubernetes.Clientset
}

func NewPods(client *kubernetes.Clientset) *Pods {
	return &Pods{
		client,
	}
}

func (pods *Pods) List(namespace string) ([]string, error) {
	podList, err := pods.client.CoreV1().Pods(namespace).List(v1.ListOptions{})

	if err != nil {
		return nil, err
	}

	var alivePods []string
	for _, pod := range podList.Items {
		if pod.DeletionTimestamp != nil {
			continue
		}
		alivePods = append(alivePods, pod.Name)
	}

	return alivePods, nil
}

func (pods *Pods) Kill(namespace string, podName string) error {
	podsInNamespace := pods.client.CoreV1().Pods(namespace)
	err := podsInNamespace.Delete(podName, &v1.DeleteOptions{})
	return err
}

func (pods *Pods) Status(namespace string, podName string) (coreV1.PodStatus, error) {
	pod, err := pods.client.CoreV1().Pods(namespace).Get(podName, v1.GetOptions{})
	return pod.Status, err
}

func (pods *Pods) extractPodNames(podList *coreV1.PodList) []string {
	var output []string
	for _, pod := range podList.Items {
		output = append(output, pod.Name)
	}
	return output
}
