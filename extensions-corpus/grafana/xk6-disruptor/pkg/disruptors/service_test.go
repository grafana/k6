package disruptors

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
)

func Test_NewServiceDisruptor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		name        string
		namespace   string
		service     *corev1.Service
		options     ServiceDisruptorOptions
		expectError bool
	}{
		{
			title:     "service exists",
			name:      "test-svc",
			namespace: "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelectorLabel("app", "test").
				WithPort("http", 80, intstr.FromInt(80)).
				BuildAsPtr(),

			options: ServiceDisruptorOptions{
				InjectTimeout: -1,
			},
			expectError: false,
		},
		{
			title:       "service does not exist",
			name:        "test-svc",
			namespace:   "test-ns",
			service:     nil,
			options:     ServiceDisruptorOptions{},
			expectError: true,
		},
		{
			title:     "empty service name",
			name:      "",
			namespace: "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelectorLabel("app", "test").
				WithPort("http", 80, intstr.FromInt(80)).
				BuildAsPtr(),
			options:     ServiceDisruptorOptions{},
			expectError: true,
		},
		{
			title:     "empty namespace",
			name:      "test-svc",
			namespace: "",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelectorLabel("app", "test").
				WithPort("http", 80, intstr.FromInt(80)).
				BuildAsPtr(),
			options:     ServiceDisruptorOptions{},
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

			client := fake.NewSimpleClientset(objs...)
			k, _ := kubernetes.NewFakeKubernetes(client)

			_, err := NewServiceDisruptor(
				t.Context(),
				k,
				tc.name,
				tc.namespace,
				tc.options,
			)

			if tc.expectError && err != nil {
				return
			}

			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed ")
				return
			}
		})
	}
}
