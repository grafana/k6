package resources

import (
	"context"
	"testing"

	"github.com/grafana/xk6-kubernetes/internal/testutils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func buildUnstructuredPod() map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "busybox",
			"namespace": "testns",
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":    "busybox",
					"image":   "busybox",
					"command": []interface{}{"sh", "-c", "sleep 30"},
				},
			},
		},
	}
}

func buildUnstructuredNamespace() map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]interface{}{
			"name": "testns",
		},
	}
}

func buildPod() *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "busybox",
			Namespace: "testns",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sh", "-c", "sleep 30"},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}
}

func buildNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "testns",
		},
	}
}

func buildNode() *corev1.Node {
	return &corev1.Node{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Node",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
		},
	}
}

func newForTest(objs ...runtime.Object) (*Client, error) {
	dynamic, err := testutils.NewFakeDynamic(objs...)
	if err != nil {
		return nil, err
	}
	return NewFromClient(context.TODO(), dynamic).WithMapper(&testutils.FakeRESTMapper{}), nil
}

func TestCreate(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		test     string
		obj      map[string]interface{}
		kind     string
		resource schema.GroupVersionResource
		name     string
		ns       string
	}{
		{
			test:     "Create Pod",
			obj:      buildUnstructuredPod(),
			kind:     "Pod",
			resource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			name:     "busybox",
			ns:       "testns",
		},
		{
			test:     "Create Namespace",
			obj:      buildUnstructuredNamespace(),
			kind:     "Namespace",
			resource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			name:     "testns",
			ns:       "",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()
			fake, _ := testutils.NewFakeDynamic()
			c := NewFromClient(context.TODO(), fake).WithMapper(&testutils.FakeRESTMapper{})

			created, err := c.Create(tc.obj)
			if err != nil {
				t.Errorf("failed %v", err)
				return
			}

			name, found, err := unstructured.NestedString(created, "metadata", "name")
			if err != nil {
				t.Errorf("error retrieving pod name %v", err)
				return
			}

			if !found {
				t.Errorf("object created has no name field")
				return
			}

			if name != tc.name {
				t.Errorf("wrong object retrieved. Expected %s Received %s", tc.name, name)
				return
			}

			// check the object was added to the fake client's object tracker
			_, err = fake.Tracker().Get(tc.resource, tc.ns, tc.name)
			if err != nil {
				t.Errorf("error retrieving object %v", err)
				return
			}
		})
	}
}

func podManifest() string {
	return `
apiVersion: v1
kind: Pod
metadata:
  name: busybox
  namespace: testns
spec:
  containers:
  - name: busybox
    image: busybox
    command: ["sleep", "300"]
`
}

func TestApply(t *testing.T) {
	// Skip test. see comments on test cases why
	t.Skip()
	t.Parallel()
	testCases := []struct {
		test     string
		manifest string
		kind     string
		name     string
		ns       string
		objects  []runtime.Object
	}{
		// This test case does not work due to https://github.com/kubernetes/client-go/issues/1184
		{
			test:     "Apply: create new pod",
			manifest: podManifest(),
			kind:     "Pod",
			name:     "busybox",
			ns:       "testns",
			objects:  []runtime.Object{},
		},
		// This test case does not work due to https://github.com/kubernetes/client-go/issues/970
		{
			test:     "Apply: existing pod",
			manifest: podManifest(),
			kind:     "Pod",
			name:     "busybox",
			ns:       "testns",
			objects: []runtime.Object{
				buildPod(),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()
			c, err := newForTest(tc.objects...)
			if err != nil {
				t.Errorf("failed %v", err)
				return
			}
			err = c.Apply(tc.manifest)
			if err != nil {
				t.Errorf("failed %v", err)
				return
			}

			obj, err := c.Get(tc.kind, tc.name, tc.ns)
			if err != nil {
				t.Errorf("failed %v", err)
				return
			}
			if obj == nil {
				t.Errorf("invalid value returned")
				return
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	// initialize with pod
	obj := buildPod()
	c, err := newForTest(obj)
	if err != nil {
		t.Errorf("failed %v", err)
		return
	}

	// set the status
	pod := buildUnstructuredPod()
	pod["status"] = map[string]interface{}{
		"phase": string(corev1.PodFailed),
	}

	updated, err := c.Update(pod)
	if err != nil {
		t.Errorf("failed %v", err)
		return
	}

	// get the status
	status, found, err := unstructured.NestedString(updated, "status", "phase")
	if err != nil {
		t.Errorf("failed %v", err)
		return
	}
	if !found || status != string(corev1.PodFailed) {
		t.Errorf("pod phase was not updated")
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		test     string
		obj      runtime.Object
		kind     string
		resource schema.GroupVersionResource
		name     string
		ns       string
	}{
		{
			test:     "Delete Pod",
			obj:      buildPod(),
			kind:     "Pod",
			resource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			name:     "busybox",
			ns:       "testns",
		},
		{
			test:     "Delete Namespace",
			obj:      buildNamespace(),
			kind:     "Namespace",
			resource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			name:     "testns",
			ns:       "",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			fake, err := testutils.NewFakeDynamic(tc.obj)
			if err != nil {
				t.Errorf("unexpected error creating fake client %v", err)
				return
			}
			c, err := NewFromClient(context.TODO(), fake).WithMapper(&testutils.FakeRESTMapper{}), nil
			if err != nil {
				t.Errorf("failed %v", err)
				return
			}

			err = c.Delete(tc.kind, tc.name, tc.ns)
			if err != nil {
				t.Errorf("failed %v", err)
				return
			}

			// check the object was added to the fake client's object tracker
			_, err = fake.Tracker().Get(tc.resource, tc.ns, tc.name)
			if !errors.IsNotFound(err) {
				t.Errorf("error retrieving object %v", err)
				return
			}
		})
	}
}

func TestGet(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		test     string
		obj      runtime.Object
		kind     string
		resource schema.GroupVersionResource
		name     string
		ns       string
	}{
		{
			test:     "Get Pod",
			obj:      buildPod(),
			kind:     "Pod",
			resource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			name:     "busybox",
			ns:       "testns",
		},
		{
			test:     "Get Namespace",
			obj:      buildNamespace(),
			kind:     "Namespace",
			resource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			name:     "testns",
			ns:       "",
		},
		{
			test:     "Get Node",
			obj:      buildNode(),
			kind:     "Node",
			resource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"},
			name:     "node1",
			ns:       "",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			fake, err := testutils.NewFakeDynamic(tc.obj)
			if err != nil {
				t.Errorf("unexpected error creating fake client %v", err)
				return
			}
			c, err := NewFromClient(context.TODO(), fake).WithMapper(&testutils.FakeRESTMapper{}), nil
			if err != nil {
				t.Errorf("failed %v", err)
				return
			}

			obj, err := c.Get(tc.kind, tc.name, tc.ns)
			if err != nil {
				t.Errorf("failed %v", err)
				return
			}

			name, found, err := unstructured.NestedString(obj, "metadata", "name")
			if err != nil {
				t.Errorf("unexpected error %v", err)
				return
			}
			if !found {
				t.Errorf("object does not have field name")
				return
			}
			if name != tc.name {
				t.Errorf("invalid pod returned. Expected %s Returned %s", tc.name, name)
				return
			}
		})
	}
}

func TestList(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		test     string
		obj      runtime.Object
		kind     string
		resource schema.GroupVersionResource
		name     string
		ns       string
	}{
		{
			test:     "List Pods",
			obj:      buildPod(),
			kind:     "Pod",
			resource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			name:     "busybox",
			ns:       "testns",
		},
		{
			test:     "List Namespace",
			obj:      buildNamespace(),
			kind:     "Namespace",
			resource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			name:     "testns",
			ns:       "",
		},
		{
			test:     "List Nodes",
			obj:      buildNode(),
			kind:     "Node",
			resource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"},
			name:     "node1",
			ns:       "",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			fake, err := testutils.NewFakeDynamic(tc.obj)
			if err != nil {
				t.Errorf("unexpected error creating fake client %v", err)
				return
			}
			c, err := NewFromClient(context.TODO(), fake).WithMapper(&testutils.FakeRESTMapper{}), nil
			if err != nil {
				t.Errorf("failed %v", err)
				return
			}

			list, err := c.List(tc.kind, tc.ns)
			if err != nil {
				t.Errorf("failed %v", err)
				return
			}

			if len(list) != 1 {
				t.Errorf("expect %d %s but %d received", 1, tc.resource.Resource, len(list))
				return
			}
		})
	}
}

func TestStructuredCreate(t *testing.T) {
	t.Parallel()

	fake, _ := testutils.NewFakeDynamic()
	c := NewFromClient(context.TODO(), fake).WithMapper(&testutils.FakeRESTMapper{})

	pod := buildPod()
	created, err := c.Structured().Create(*pod)
	if err != nil {
		t.Errorf("failed %v", err)
		return
	}

	createdPod, ok := created.(corev1.Pod)
	if !ok {
		t.Errorf("invalid type assertion")
	}
	if createdPod.Name != pod.Name {
		t.Errorf("invalid pod returned. Expected %s Returned %s", pod.Name, createdPod.Name)
		return
	}
}

func TestStructuredGet(t *testing.T) {
	t.Parallel()
	// initialize with pod
	initPod := buildPod()
	c, err := newForTest(initPod)
	if err != nil {
		t.Errorf("failed %v", err)
		return
	}

	pod := &corev1.Pod{}
	err = c.Structured().Get("Pod", "busybox", "testns", pod)
	if err != nil {
		t.Errorf("failed %v", err)
		return
	}
	if pod.Name != initPod.Name {
		t.Errorf("invalid pod returned. Expected %s Returned %s", initPod.Name, pod.Name)
		return
	}
}

func TestStructuredList(t *testing.T) {
	t.Parallel()
	// initialize with pod
	pod := buildPod()
	c, err := newForTest(pod)
	if err != nil {
		t.Errorf("failed %v", err)
		return
	}

	podList := []corev1.Pod{}
	err = c.Structured().List("Pod", "testns", &podList)
	if err != nil {
		t.Errorf("failed %v", err)
		return
	}

	if len(podList) != 1 {
		t.Errorf("one pod expected but %d returned", len(podList))
		return
	}

	if podList[0].Name != pod.Name {
		t.Errorf("invalid pod returned. Expected %s Returned %s", pod.Name, podList[0].Name)
		return
	}
}

func TestStructuredDelete(t *testing.T) {
	t.Parallel()
	// initialize with pod
	obj := buildPod()
	c, err := newForTest(obj)
	if err != nil {
		t.Errorf("failed %v", err)
		return
	}

	err = c.Structured().Delete("Pod", "busybox", "testns")
	if err != nil {
		t.Errorf("failed %v", err)
		return
	}
}

func TestStructuredUpdate(t *testing.T) {
	t.Parallel()
	// initialize with pod
	pod := buildPod()
	c, err := newForTest(pod)
	if err != nil {
		t.Errorf("failed %v", err)
		return
	}

	// change status
	pod.Status.Phase = corev1.PodFailed
	updated, err := c.Structured().Update(*pod)
	if err != nil {
		t.Errorf("failed %v", err)
		return
	}

	updatedPod, ok := updated.(corev1.Pod)
	if !ok {
		t.Errorf("invalid type assertion")
	}
	status := updatedPod.Status.Phase
	if status != corev1.PodFailed {
		t.Errorf("pod status not updated")
		return
	}
}
