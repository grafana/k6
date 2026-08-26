package disruptors

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
	"github.com/grafana/xk6-disruptor/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/client-go/kubernetes/fake"
)

func Test_NewPodSelector(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		spec        PodSelectorSpec
		expectError bool
		expected    []string
	}{
		{
			title: "valid specs",
			spec: PodSelectorSpec{
				Namespace: "test-ns",
				Select: PodAttributes{Labels: map[string]string{
					"app": "test",
				}},
			},
			expectError: false,
		},
		{
			title:       "empty specs",
			spec:        PodSelectorSpec{},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			client := fake.NewSimpleClientset()
			k, _ := kubernetes.NewFakeKubernetes(client)
			helper := k.PodHelper(tc.spec.Namespace)

			_, err := NewPodSelector(tc.spec, helper)

			if tc.expectError && err != nil {
				return
			}

			if !tc.expectError && err != nil {
				t.Fatalf("unexpected error creating pod selector: %v", err)
			}

			if tc.expectError && err == nil {
				t.Fatalf("should had failed creating pod selector")
			}
		})
	}
}

func Test_PodSelectorString(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		selector PodSelectorSpec
		expected string
	}{
		{
			name:     "Empty selector",
			expected: `all pods in ns "default"`,
		},
		{
			name: "Only inclusions",
			selector: PodSelectorSpec{
				Namespace: "testns",
				Select:    PodAttributes{map[string]string{"foo": "bar"}},
			},
			expected: `pods including(foo=bar) in ns "testns"`,
		},
		{
			name: "Only exclusions",
			selector: PodSelectorSpec{
				Namespace: "testns",
				Exclude:   PodAttributes{map[string]string{"foo": "bar"}},
			},
			expected: `pods excluding(foo=bar) in ns "testns"`,
		},
		{
			name: "Both inclusions and exclusions",
			selector: PodSelectorSpec{
				Namespace: "testns",
				Select:    PodAttributes{map[string]string{"foo": "bar"}},
				Exclude:   PodAttributes{map[string]string{"boo": "baa"}},
			},
			expected: `pods including(foo=bar), excluding(boo=baa) in ns "testns"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			output := tc.selector.String()
			if tc.expected != output {
				t.Fatalf("expected string does not match output string:\n%s\n%s", tc.expected, output)
			}
		})
	}
}

func Test_PodSelectorTargets(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		namespace   string
		pods        []corev1.Pod
		spec        PodSelectorSpec
		expectError bool
		expected    []string
	}{
		{
			title:     "matching pods",
			namespace: "test-ns",
			pods: []corev1.Pod{
				builders.NewPodBuilder("pod-1").
					WithNamespace("test-ns").
					WithLabel("app", "test").
					Build(),
			},
			spec: PodSelectorSpec{
				Namespace: "test-ns",
				Select: PodAttributes{Labels: map[string]string{
					"app": "test",
				}},
			},
			expectError: false,
			expected:    []string{"pod-1"},
		},
		{
			title:     "no matching pods",
			namespace: "test-ns",
			pods:      []corev1.Pod{},
			spec: PodSelectorSpec{
				Namespace: "test-ns",
				Select: PodAttributes{Labels: map[string]string{
					"app": "test",
				}},
			},
			expected:    nil,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			var objs []runtime.Object
			for p := range tc.pods {
				objs = append(objs, &tc.pods[p])
			}

			client := fake.NewSimpleClientset(objs...)
			k, _ := kubernetes.NewFakeKubernetes(client)

			s, err := NewPodSelector(tc.spec, k.PodHelper(tc.namespace))
			if err != nil {
				t.Fatalf("failed%v", err)
			}

			targets, err := s.Targets(t.Context())
			if tc.expectError && err != nil {
				return
			}

			if !tc.expectError && err != nil {
				t.Fatalf("failed%v", err)
			}

			if tc.expectError && err == nil {
				t.Fatalf("should had failed")
			}

			targetNames := utils.PodNames(targets)
			sort.Strings(targetNames)
			if diff := cmp.Diff(targetNames, tc.expected); diff != "" {
				t.Fatalf("expected targets dot not match returned\n%s", diff)
			}
		})
	}
}

func Test_ServicePodSelectorTargets(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		name        string
		namespace   string
		service     *corev1.Service
		pods        []corev1.Pod
		expectError bool
		expected    []string
	}{
		{
			title:     "one endpoint",
			name:      "test-svc",
			namespace: "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelectorLabel("app", "test").
				WithPort("http", 80, intstr.FromInt(80)).
				BuildAsPtr(),
			pods: []corev1.Pod{
				builders.NewPodBuilder("pod-1").
					WithNamespace("test-ns").
					WithLabel("app", "test").
					Build(),
			},
			expectError: false,
			expected:    []string{"pod-1"},
		},
		{
			title:     "multiple endpoints",
			name:      "test-svc",
			namespace: "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelectorLabel("app", "test").
				WithPort("http", 80, intstr.FromInt(80)).
				BuildAsPtr(),
			pods: []corev1.Pod{
				builders.NewPodBuilder("pod-1").
					WithNamespace("test-ns").
					WithLabel("app", "test").
					Build(),
				builders.NewPodBuilder("pod-2").
					WithNamespace("test-ns").
					WithLabel("app", "test").
					Build(),
			},
			expectError: false,
			expected:    []string{"pod-1", "pod-2"},
		},
		{
			title:     "no endpoints",
			name:      "test-svc",
			namespace: "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelectorLabel("app", "test").
				WithPort("http", 80, intstr.FromInt(80)).
				BuildAsPtr(),
			pods:        nil,
			expectError: true,
		},
		{
			title:       "service does not exist",
			name:        "test-svc",
			namespace:   "test-ns",
			service:     nil,
			pods:        nil,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			objs := []runtime.Object{}
			if tc.service != nil {
				objs = append(objs, tc.service)
			}
			for p := range tc.pods {
				objs = append(objs, &tc.pods[p])
			}

			client := fake.NewSimpleClientset(objs...)
			k, _ := kubernetes.NewFakeKubernetes(client)

			d, err := NewServicePodSelector(
				tc.name,
				tc.namespace,
				k.ServiceHelper(tc.namespace),
			)
			if err != nil {
				t.Fatalf("failed%v", err)
			}

			targets, err := d.Targets(t.Context())

			if tc.expectError && err != nil {
				return
			}

			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			targetNames := utils.PodNames(targets)
			sort.Strings(targetNames)
			if diff := cmp.Diff(targetNames, tc.expected); diff != "" {
				t.Errorf("expected targets dot not match returned\n%s", diff)
				return
			}
		})
	}
}
