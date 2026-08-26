package namespace

import (
	"fmt"
	"regexp"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// generates name for a resource
func generateName(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
	//nolint:forcetypeassert
	ret = action.(k8stesting.CreateAction).GetObject()
	meta, ok := ret.(metav1.Object)
	if !ok {
		return
	}

	if meta.GetName() == "" && meta.GetGenerateName() != "" {
		meta.SetName(meta.GetGenerateName() + rand.String(5))
	}

	return
}

func Test_CreateNamespace(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		options     []TestNamespaceOption
		expectError bool
		check       func(kubernetes.Interface, string) error
	}{
		{
			title:       "default options",
			options:     []TestNamespaceOption{},
			expectError: false,
			check: func(_ kubernetes.Interface, ns string) error {
				match, _ := regexp.MatchString(`testns-[a-z|0-9]+`, ns)
				if !match {
					return fmt.Errorf("expected pattern 'testns-xxxxx' got %q", ns)
				}
				return nil
			},
		},
		{
			title:       "fixed name",
			options:     []TestNamespaceOption{WithName("testns")},
			expectError: false,
			check: func(_ kubernetes.Interface, ns string) error {
				if ns != "testns" {
					return fmt.Errorf("expected 'testns' got %q", ns)
				}

				return nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			client := fake.NewSimpleClientset()
			// Fake client does not support generated named
			client.PrependReactor("create", "*", generateName)

			ns, err := CreateTestNamespace(t.Context(), t, client, tc.options...)
			if err != nil && !tc.expectError {
				t.Errorf("unexpected error %v", err)
				return
			}

			if err == nil && tc.expectError {
				t.Errorf("should had failed")
				return
			}

			if err != nil && tc.expectError {
				return
			}

			err = tc.check(client, ns)
			if err != nil {
				t.Errorf("%v", err)
				return
			}
		})
	}
}
