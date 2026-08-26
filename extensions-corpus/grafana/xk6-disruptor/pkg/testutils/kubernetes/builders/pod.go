package builders

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodBuilder defines the methods for building a Pod
type PodBuilder interface {
	// Build returns a Pod with the attributes defined in the PodBuilder
	Build() corev1.Pod
	// WithNamespace sets namespace for the pod to be built
	WithNamespace(namespace string) PodBuilder
	// WithDefaultNamespace sets namespace for the pod as "default"
	// By default, the Pod has no namespace set to allow overriding it when creating the resource in k8s
	WithDefaultNamespace() PodBuilder
	// WithLabels sets the labels to the pod (overrides any previously set labels)
	WithLabels(labels map[string]string) PodBuilder
	// WithLabel adds a label to the Pod
	WithLabel(name string, value string) PodBuilder
	// WithAnnotation adds an annotation
	WithAnnotation(name string, value string) PodBuilder
	// WithPhase sets the PodPhase for the pod to be built
	WithPhase(status corev1.PodPhase) PodBuilder
	// WithIP sets the IP address for the pod to be built
	WithIP(ip string) PodBuilder
	// WithHostNetwork sets the hostNetwork property of the pod to be built
	WithHostNetwork(hostNetwork bool) PodBuilder
	// WithContainer add a container to the pod
	WithContainer(c corev1.Container) PodBuilder
}

// podBuilder defines the attributes for building a pod
type podBuilder struct {
	name        string
	namespace   string
	labels      map[string]string
	annotations map[string]string
	phase       corev1.PodPhase
	ip          string
	hostNetwork bool
	containers  []corev1.Container
}

// NewPodBuilder creates a new instance of PodBuilder with the given pod name
// and default attributes such as containers and namespace
func NewPodBuilder(name string) PodBuilder {
	return &podBuilder{
		name:        name,
		annotations: map[string]string{},
		labels:      map[string]string{},
	}
}

func (b *podBuilder) WithNamespace(namespace string) PodBuilder {
	b.namespace = namespace
	return b
}

func (b *podBuilder) WithDefaultNamespace() PodBuilder {
	b.namespace = metav1.NamespaceDefault
	return b
}

func (b *podBuilder) WithPhase(phase corev1.PodPhase) PodBuilder {
	b.phase = phase
	return b
}

func (b *podBuilder) WithIP(ip string) PodBuilder {
	b.ip = ip
	return b
}

func (b *podBuilder) WithLabels(labels map[string]string) PodBuilder {
	b.labels = labels
	return b
}

func (b *podBuilder) WithLabel(name string, value string) PodBuilder {
	b.labels[name] = value
	return b
}

func (b *podBuilder) WithAnnotation(name string, value string) PodBuilder {
	b.annotations[name] = value
	return b
}

func (b *podBuilder) WithHostNetwork(hostNetwork bool) PodBuilder {
	b.hostNetwork = hostNetwork
	return b
}

func (b *podBuilder) WithContainer(c corev1.Container) PodBuilder {
	b.containers = append(b.containers, c)
	return b
}

func (b *podBuilder) Build() corev1.Pod {
	pod := corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        b.name,
			Namespace:   b.namespace,
			Labels:      b.labels,
			Annotations: b.annotations,
		},
		Spec: corev1.PodSpec{
			Containers:          b.containers,
			HostNetwork:         b.hostNetwork,
			EphemeralContainers: nil,
		},
		Status: corev1.PodStatus{
			Phase: b.phase,
		},
	}

	// PodIPs is a patchMergeKey field, so it should be nil if no IPs are present. Otherwise, creation of
	// StrategicMerge patches will fail with:
	// map: map[] does not contain declared merge key: ip
	if b.ip != "" {
		pod.Status.PodIP = b.ip
		pod.Status.PodIPs = []corev1.PodIP{{IP: b.ip}}
	}

	return pod
}
