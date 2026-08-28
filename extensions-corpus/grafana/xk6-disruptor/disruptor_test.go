package disruptor

import (
	"context"
	"testing"

	"github.com/grafana/sobek"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
	extensionapi "go.k6.io/k6-extension-api"

	"k8s.io/client-go/kubernetes/fake"
)

// testVU creates a test VU
type testVUImpl struct {
	ctx context.Context
	rt  *sobek.Runtime
}

func (v *testVUImpl) Context() context.Context { return v.ctx }
func (v *testVUImpl) Runtime() *sobek.Runtime  { return v.rt }

func testVU() extensionapi.VU {
	rt := sobek.New()

	return &testVUImpl{rt: rt, ctx: context.Background()}
}

// instantiates a module with a fake kubernetes and a test VU
func setTestModule(k8s *kubernetes.FakeKubernetes, vu extensionapi.VU) error {
	m := ModuleInstance{
		k8s: k8s,
		vu:  vu,
	}
	err := vu.Runtime().Set("PodDisruptor", m.Exports().Named["PodDisruptor"])
	if err != nil {
		return err
	}
	err = vu.Runtime().Set("ServiceDisruptor", m.Exports().Named["ServiceDisruptor"])

	return err
}

const listTargetsScript = `
const selector = {
   namespace: "default",
   select: {
     labels: {
	app: "test"
     }
   }
} 
const opts = {
	injectTimeout: "-1s"
}
const disruptor = new PodDisruptor(selector, opts)
const targets = disruptor.targets()
if (targets.length != 1) {
   throw new Error("expected list to have one target")
} 
`

func Test_PodDisruptor(t *testing.T) {
	t.Parallel()

	pod := builders.NewPodBuilder("pod-with-app-label").
		WithDefaultNamespace().
		WithLabel("app", "test").
		Build()
	client := fake.NewSimpleClientset(&pod)
	k8s, _ := kubernetes.NewFakeKubernetes(client)
	vu := testVU()
	err := setTestModule(k8s, vu)
	if err != nil {
		t.Errorf("test setup failed: %v", err)
	}

	_, err = vu.Runtime().RunString(listTargetsScript)
	if err != nil {
		t.Errorf("failed %v", err)
	}
}

const listServiceTargetsScript = `
// force no waiting for ephemeral container as the mock will not update its status
const opts = {
	injectTimeout: "-1s"
}
const disruptor = new ServiceDisruptor("app-service", "default", opts)
const targets = disruptor.targets()
if (targets.length != 1) {
   throw new Error("expected list to have one target")
} 
`

func Test_ServiceDisruptor(t *testing.T) {
	t.Parallel()
	labels := map[string]string{
		"app": "test",
	}
	pod := builders.NewPodBuilder("app-pod").
		WithDefaultNamespace().
		WithLabels(labels).
		Build()
	svc := builders.NewServiceBuilder("app-service").
		WithNamespace("default").
		WithSelector(labels).
		Build()

	client := fake.NewSimpleClientset(&pod, &svc)
	k8s, _ := kubernetes.NewFakeKubernetes(client)
	vu := testVU()
	err := setTestModule(k8s, vu)
	if err != nil {
		t.Errorf("test setup failed: %v", err)
	}

	_, err = vu.Runtime().RunString(listServiceTargetsScript)
	if err != nil {
		t.Errorf("failed %v", err)
	}
}
