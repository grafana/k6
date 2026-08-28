package kubernetes

import (
	"testing"

	"github.com/grafana/sobek"
	localutils "github.com/grafana/xk6-kubernetes/internal/testutils"
	"github.com/stretchr/testify/require"
	extensionapitest "go.k6.io/k6-extension-api/test"
	"k8s.io/apimachinery/pkg/runtime"
)

// setupTestEnv should be called from each test to build the execution environment for the test
func setupTestEnv(t *testing.T, objs ...runtime.Object) *sobek.Runtime {
	vu := extensionapitest.NewVU()
	rt := vu.Runtime()

	root := &RootModule{}
	m, ok := root.NewModuleInstance(
		vu,
	).(*ModuleInstance)
	require.True(t, ok)
	require.NoError(t, rt.Set("Kubernetes", m.Exports().Named["Kubernetes"]))

	m.clientset = localutils.NewFakeClientset(objs...)

	dynamic, err := localutils.NewFakeDynamic()
	if err != nil {
		t.Errorf("unexpected error creating fake client %v", err)
	}
	m.dynamic = dynamic
	m.mapper = &localutils.FakeRESTMapper{}

	return rt
}

// TestGenericApiIsScriptable runs through creating, getting, listing and deleting an object
func TestGenericApiIsScriptable(t *testing.T) {
	t.Parallel()

	rt := setupTestEnv(t)

	_, err := rt.RunString(`
const k8s = new Kubernetes()

const podSpec = {
    apiVersion: "v1",
    kind:       "Pod",
    metadata: {
        name:      "busybox",
        namespace: "testns"
    },
    spec: {
        containers: [
            {
                name:    "busybox",
                image:   "busybox",
                command: ["sh", "-c", "sleep 30"]
            }
        ]
    }
}

var created = k8s.create(podSpec)

var pod = k8s.get(podSpec.kind, podSpec.metadata.name, podSpec.metadata.namespace)
if (podSpec.metadata.name != pod.metadata.name) {
	throw new Error("Fetch by name did not return the Service. Expected: " + podSpec.metadata.name + " but got: " + fetched.name)
}

const pods = k8s.list(podSpec.kind, podSpec.metadata.namespace)
if (pods === undefined || pods.length < 1) {
	throw new Error("Expected listing with 1 Pod")
}

k8s.delete(podSpec.kind, podSpec.metadata.name, podSpec.metadata.namespace)
if (k8s.list(podSpec.kind, podSpec.metadata.namespace).length != 0) {
	throw new Error("Deletion failed to remove pod")
}
`)
	require.NoError(t, err)
}

// TestHelpersAreScriptable runs helpers
func TestHelpersAreScriptable(t *testing.T) {
	t.Parallel()

	rt := setupTestEnv(t)

	_, err := rt.RunString(`
const k8s = new Kubernetes()

let pod = {
	apiVersion: "v1",
	kind:       "Pod",
	metadata: {
	    name:      "busybox",
	    namespace:  "default"
	},
	spec: {
	    containers: [
		{
		    name:    "busybox",
		    image:   "busybox",
		    command: ["sh", "-c", "sleep 30"]
		}
	    ]
	},
	status: {
		phase: "Running"
	}
}

// create pod in test namespace
k8s.create(pod)

// get helpers for test namespace
const helpers = k8s.helpers()

// wait for pod to be running
if (!helpers.waitPodRunning(pod.metadata.name, 5)) {
	throw new Error("should not timeout")
}
`)
	require.NoError(t, err)
}
