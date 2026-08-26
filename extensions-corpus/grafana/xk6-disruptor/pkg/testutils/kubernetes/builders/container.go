package builders

import corev1 "k8s.io/api/core/v1"

// ContainerBuilder defines the methods for building a Container
type ContainerBuilder interface {
	// WithPort adds a port to the container
	WithPort(name string, port int32) ContainerBuilder
	// WithImage sets the container's image
	WithImage(image string) ContainerBuilder
	// WithPullPolicy sets the container's image pull policy (default is IfNotPresent)
	WithPullPolicy(policy corev1.PullPolicy) ContainerBuilder
	// WithCommand sets the container's command
	WithCommand(cmd ...string) ContainerBuilder
	// WithCapabilites adds capabilities to the container's security context
	WithCapabilities(capabilities ...corev1.Capability) ContainerBuilder
	// Build returns a Pod with the attributes defined in the PodBuilder
	Build() corev1.Container
	// WithEnvVarFromField adds an environment variable to the container
	WithEnvVar(name string, value string) ContainerBuilder
	// WithEnvVarFromField adds an environment variable to the container referencing a field
	// Example: "PodName", "metadata.name"
	WithEnvVarFromField(name string, path string) ContainerBuilder
}

// containerBuilder maintains the configuration for building a container
type containerBuilder struct {
	name         string
	image        string
	imagePolicy  corev1.PullPolicy
	command      []string
	ports        []corev1.ContainerPort
	capabilities []corev1.Capability
	vars         []corev1.EnvVar
}

// NewContainerBuilder returns a new ContainerBuilder
func NewContainerBuilder(name string) ContainerBuilder {
	return &containerBuilder{
		name:        name,
		imagePolicy: corev1.PullIfNotPresent,
	}
}

func (b *containerBuilder) WithPort(name string, port int32) ContainerBuilder {
	b.ports = append(
		b.ports,
		corev1.ContainerPort{
			Name:          name,
			ContainerPort: port,
		},
	)
	return b
}

func (b *containerBuilder) WithImage(image string) ContainerBuilder {
	b.image = image
	return b
}

func (b *containerBuilder) WithPullPolicy(policy corev1.PullPolicy) ContainerBuilder {
	b.imagePolicy = policy
	return b
}

func (b *containerBuilder) WithCommand(command ...string) ContainerBuilder {
	b.command = command
	return b
}

func (b *containerBuilder) WithCapabilities(capabilities ...corev1.Capability) ContainerBuilder {
	b.capabilities = append(b.capabilities, capabilities...)
	return b
}

func (b *containerBuilder) WithEnvVar(name string, value string) ContainerBuilder {
	b.vars = append(b.vars, corev1.EnvVar{Name: name, Value: value})
	return b
}

func (b *containerBuilder) WithEnvVarFromField(name string, path string) ContainerBuilder {
	valueFrom := &corev1.EnvVarSource{
		FieldRef: &corev1.ObjectFieldSelector{FieldPath: path},
	}
	b.vars = append(b.vars, corev1.EnvVar{Name: name, ValueFrom: valueFrom})

	return b
}

func (b *containerBuilder) Build() corev1.Container {
	return corev1.Container{
		Name:            b.name,
		Image:           b.image,
		ImagePullPolicy: b.imagePolicy,
		Command:         b.command,
		Ports:           b.ports,
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: b.capabilities,
			},
		},
		Env: b.vars,
	}
}
