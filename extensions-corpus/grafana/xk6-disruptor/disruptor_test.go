package disruptor

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/grafana/sobek"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
	"github.com/sirupsen/logrus"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"

	"k8s.io/client-go/kubernetes/fake"
)

// testVU creates a test VU
func testVU() modules.VU {
	rt := sobek.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	testLog := logrus.New()
	testLog.SetOutput(ioutil.Discard)

	state := &lib.State{
		Options: lib.Options{
			SystemTags: metrics.NewSystemTagSet(metrics.TagVU),
		},
		Logger: testLog,
	}

	return &modulestest.VU{
		RuntimeField: rt,
		InitEnvField: &common.InitEnvironment{},
		CtxField:     context.Background(),
		StateField:   state,
	}
}

// instantiates a module with a fake kubernetes and a test VU
func setTestModule(k8s *kubernetes.FakeKubernetes, vu modules.VU) error {
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
