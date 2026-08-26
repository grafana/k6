package api

import (
	"context"
	"fmt"
	"testing"

	"github.com/grafana/sobek"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
	"go.k6.io/k6/js/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

// test environment
type testEnv struct {
	rt     *sobek.Runtime
	client *fake.Clientset
	k8s    kubernetes.Kubernetes
}

// a function that constructs an object
type constructor func(*testEnv, sobek.ConstructorCall) (*sobek.Object, error)

// registers a constructor with a name in the environment's runtime
func (env *testEnv) registerConstructor(name string, constructor constructor) error {
	var object *sobek.Object
	var err error
	err = env.rt.Set(name, func(c sobek.ConstructorCall) *sobek.Object {
		object, err = constructor(env, c)
		if err != nil {
			common.Throw(env.rt, fmt.Errorf("error creating %s: %w", name, err))
		}
		return object
	})
	return err
}

func testSetup() (*testEnv, error) {
	rt := sobek.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	client := fake.NewSimpleClientset()
	k8s, err := kubernetes.NewFakeKubernetes(client)
	if err != nil {
		return nil, err
	}

	// Create a Service and a backing pod that match the targets in fault injection methods
	// We are not testing the fault injection logic, only the arguments passed to the API but
	// those methods would fail if they don't find a target pod.
	// Note: the ServiceDisruptor constructor also will fail if the target service doesn't exist
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "namespace"},
	}

	_, err = k8s.Client().CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating namespace: %w", err)
	}

	pod := builders.NewPodBuilder("some-pod").
		WithNamespace(ns.Name).
		WithLabel("app", "app").
		WithContainer(builders.NewContainerBuilder("main").
			WithPort("http", 80).
			WithImage("fake.registry.local/main").
			Build(),
		).
		WithIP("192.0.2.6").
		Build()

	// ServiceDisruptor and PodDisruptor will also attempt to inject the disruptor agent into a target
	// pod once it's discovered, and then wait for that container to be Running. Flagging this pod as ready is hard to
	// do with the k8s fake client, so we take advantage of the fact that both injection and check are skipped if the
	// agent container already exists by creating the fake pod with the sidecar already added.
	agentContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:  "xk6-agent",
			Image: "fake.registry.local/xk6-agent",
		},
	}

	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, agentContainer)

	_, err = k8s.Client().CoreV1().Pods(ns.Name).Create(context.TODO(), &pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating namespace: %w", err)
	}

	svc := builders.NewServiceBuilder("some-service").
		WithNamespace(ns.Name).
		WithSelectorLabel("app", "app").
		WithPort("http", 80, intstr.FromString("http")).
		Build()

	_, err = k8s.Client().CoreV1().Services(ns.Name).Create(context.TODO(), &svc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating namespace: %w", err)
	}

	return &testEnv{
		rt:     rt,
		client: client,
		k8s:    k8s,
	}, nil
}

func Test_PodDisruptorConstructor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description string
		script      string
		expectError bool
	}{
		{
			description: "valid constructor",
			script: `
			const selector = {
				namespace: "namespace",
				select: {
					labels: {
						app: "app"
					}
				}
			}
			const opts = {
				injectTimeout: "10s"
			}
			new PodDisruptor(selector, opts)
			`,
			expectError: false,
		},
		{
			description: "valid constructor without options",
			script: `
			const selector = {
				namespace: "namespace",
				select: {
					labels: {
						app: "app"
					}
				}
			}
			new PodDisruptor(selector)
			`,
			expectError: false,
		},
		{
			description: "invalid constructor without selector",
			script: `
			new PodDisruptor()
			`,
			expectError: true,
		},
		{
			description: "invalid constructor with malformed selector",
			script: `
			const selector = {
				namespace: "namespace",
				labels: {
					app: "app"
				}
			}
			new PodDisruptor(selector)
			`,
			expectError: true,
		},
		{
			description: "invalid constructor with malformed options",
			script: `
			const selector = {
				namespace: "namespace",
				select: {
					labels: {
						app: "app"
					}
				}
			}
			const opts = {
				timeout: "0s"
			}
			new PodDisruptor(selector, opts)
			`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			env, err := testSetup()
			if err != nil {
				t.Errorf("error in test setup %v", err)
				return
			}

			err = env.registerConstructor("PodDisruptor", func(e *testEnv, c sobek.ConstructorCall) (*sobek.Object, error) {
				return NewPodDisruptor(t.Context(), e.rt, c, e.k8s)
			})
			if err != nil {
				t.Errorf("error in test setup %v", err)
				return
			}

			_, err = env.rt.RunString(tc.script)

			if !tc.expectError && err != nil {
				t.Errorf("failed %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}
		})
	}
}

const setupPodDisruptor = `
	const selector = {
	namespace: "namespace",
	select: {
		labels: {
			app: "app"
		}
	}
}

const d = new PodDisruptor(selector)
`

// This function tests covers both PodDisruptor and ServiceDisruptor because
// they are wrapped by the same methods in the API.
func Test_JsPodDisruptor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description string
		script      string
		expectError bool
	}{
		{
			description: "get targets",
			script: `
			d.targets()
			`,
			expectError: false,
		},
		{
			description: "inject HTTP Fault with full arguments",
			script: `
			const fault = {
				errorRate: 1.0,
				errorCode: 500,
				averageDelay: "100ms",
				delayVariation: "10ms",
				errorBody: '',
				exclude: "",
				port: 80
			}

			const faultOpts = {
				proxyPort: 8080,
			}

			d.injectHTTPFaults(fault, "1s", faultOpts)
			`,
			expectError: false,
		},
		{
			description: "inject HTTP Fault without options",
			script: `
			const fault = {
				errorRate: 1.0,
				errorCode: 500,
				averageDelay: "100ms",
				delayVariation: "10ms",
				errorBody: '',
				exclude: "",
				port: 80
			}

			d.injectHTTPFaults(fault, "1s")
			`,
			expectError: false,
		},
		{
			description: "inject HTTP Fault without duration",
			script: `
			const fault = {
				errorRate: 1.0,
				errorCode: 500,
				averageDelay: "100ms",
				delayVariation: "10ms",
				errorBody: '',
				exclude: "",
				port: 80
			}

			d.injectHTTPFaults(fault)
			`,
			expectError: true,
		},
		{
			description: "inject HTTP Fault with invalid duration",
			script: `
			const fault = {
				errorRate: 1.0,
				errorCode: 500,
				averageDelay: "100ms",
				delayVariation: "10ms",
				errorBody: '',
				exclude: "",
				port: 80
			}

			d.injectHTTPFaults(fault, "1")  // missing duration unit
			`,
			expectError: true,
		},
		{
			description: "inject HTTP Fault with malformed fault (misspelled field)",
			script: `
			const fault = {
				errorRate: 1.0,
				error: 500,       // this is should be 'errorCode'
			}

			d.injectHTTPFaults(fault, "1s")
			`,
			expectError: true,
		},
		{
			description: "inject Grpc Fault with full arguments",
			script: `
			const fault = {
				errorRate: 1.0,
				statusCode: 500,
				statusMessage: '',
				averageDelay: "100ms",
				delayVariation: "10ms",
				exclude: "",
				port: 80
			}

			const faultOpts = {
				proxyPort: 4000,
			}

			d.injectGrpcFaults(fault, "1s", faultOpts)
			`,
			expectError: false,
		},
		{
			description: "inject Grpc Fault without options",
			script: `
			const fault = {
				errorRate: 1.0,
				statusCode: 500,
				statusMessage: '',
				averageDelay: "100ms",
				delayVariation: "10ms",
				exclude: "",
				port: 80
			}

			d.injectGrpcFaults(fault, "1s")
			`,
			expectError: false,
		},
		{
			description: "inject Grpc Fault without duration",
			script: `
			const fault = {
				errorRate: 1.0,
				statusCode: 500,
				statusMessage: '',
				averageDelay: "100ms",
				delayVariation: "10ms",
				exclude: "",
				port: 80
			}

			const faultOpts = {
				proxyPort: 4000,
			}

			d.injectGrpcFaults(fault)
			`,
			expectError: true,
		},
		{
			description: "inject Grpc Fault without invalid duration",
			script: `
			const fault = {
				errorRate: 1.0,
				statusCode: 500,
				statusMessage: '',
				averageDelay: "100ms",
				delayVariation: "10ms",
				exclude: "",
				port: 80
			}

			const faultOpts = {
				proxyPort: 4000,
			}

			d.injectGrpcFaults(fault, "1")    // missing duration unit
			`,
			expectError: true,
		},
		{
			description: "inject Grpc Fault with malformed fault (misspelled field)",
			script: `
			const fault = {
				errorRate: 1.0,
				status: 500,       // this is should be 'statusCode'
			}

			d.injectGrpcFaults(fault, "1m")
			`,
			expectError: true,
		},
		{
			description: "Terminate Pods (integer count)",
			script: `
			const fault = {
				count: 1
			}

			d.terminatePods(fault)
			`,
			expectError: false,
		},
		{
			description: "Terminate Pods (percentage count)",
			script: `
			const fault = {
				count: '100%'
			}

			d.terminatePods(fault)
			`,
			expectError: false,
		},
		{
			description: "Terminate Pods (invalid percentage count)",
			script: `
			const fault = {
				count: '100'
			}

			d.terminatePods(fault)
			`,
			expectError: true,
		},
		{
			description: "Terminate Pods (missing argument)",
			script: `
			d.terminatePods()
			`,
			expectError: true,
		},
		{
			description: "Terminate Pods (empty argument)",
			script: `
			d.terminatePods({})
			`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			env, err := testSetup()
			if err != nil {
				t.Errorf("error in test setup %v", err)
				return
			}

			err = env.registerConstructor("PodDisruptor", func(e *testEnv, c sobek.ConstructorCall) (*sobek.Object, error) {
				return NewPodDisruptor(t.Context(), e.rt, c, e.k8s)
			})
			if err != nil {
				t.Errorf("error in test setup %v", err)
				return
			}

			_, err = env.rt.RunString(setupPodDisruptor)
			if err != nil {
				t.Errorf("error in test setup %v", err)
				return
			}

			_, err = env.rt.RunString(tc.script)

			if !tc.expectError && err != nil {
				t.Errorf("failed %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}
		})
	}
}

func Test_ServiceDisruptorConstructor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description string
		script      string
		expectError bool
	}{
		{
			description: "valid constructor",
			script: `
			const opts = {
				injectTimeout: "30s"
			}
			new ServiceDisruptor("some-service", "namespace", opts)
			`,
			expectError: false,
		},
		{
			description: "valid constructor without options",
			script: `
			new ServiceDisruptor("some-service", "namespace")
			`,
			expectError: false,
		},
		{
			description: "invalid constructor without namespace",
			script: `
			new ServiceDisruptor("some-service")
			`,
			expectError: true,
		},
		{
			description: "invalid constructor service name is not string",
			script: `
			new ServiceDisruptor(1, "namespace")
			`,
			expectError: true,
		},
		{
			description: "invalid constructor namespace is not string",
			script: `
			new ServiceDisruptor("some-service", {})
			`,
			expectError: true,
		},
		{
			description: "invalid constructor without arguments",
			script: `
			new ServiceDisruptor()
			`,
			expectError: true,
		},
		{
			description: "valid constructor malformed options",
			script: `
			const opts = {
				timeout: "30s"
			}
			new ServiceDisruptor("some-service", "namespace", opts)
			`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			env, err := testSetup()
			if err != nil {
				t.Errorf("error in test setup %v", err)
				return
			}

			err = env.registerConstructor("ServiceDisruptor", func(e *testEnv, c sobek.ConstructorCall) (*sobek.Object, error) {
				return NewServiceDisruptor(t.Context(), e.rt, c, e.k8s)
			})
			if err != nil {
				t.Errorf("error in test setup %v", err)
				return
			}

			// create a k8s resources because the ServiceDisruptor's constructor expects it to exist
			labels := map[string]string{
				"app": "test",
			}
			svc := builders.NewServiceBuilder("service").WithSelector(labels).Build()
			_, _ = env.client.CoreV1().Services("default").Create(t.Context(), &svc, metav1.CreateOptions{})

			_, err = env.rt.RunString(tc.script)

			if !tc.expectError && err != nil {
				t.Errorf("failed %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}
		})
	}
}
