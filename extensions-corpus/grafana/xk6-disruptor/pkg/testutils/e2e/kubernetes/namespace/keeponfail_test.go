//go:build keeponfail
// +build keeponfail

package namespace

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// Test_KeepOnFail is expected to fail as it tests the preservation of namespace
// in case of a test failure
// It only runs if the TEST_KEEPONFAIL=true is specified.
func Test_KeepOnFail(t *testing.T) {
	client := fake.NewSimpleClientset()

	t.Run("Sub-test", func(t *testing.T) {
		_, err := CreateTestNamespace(
			context.TODO(),
			t,
			client,
			WithKeepOnFail(true),
			WithName("testns"),
		)
		if err != nil {
			t.Errorf("unexpected error %v", err)
			return
		}

		t.Fail()
	})

	_, err := client.CoreV1().Namespaces().Get(context.TODO(), "testns", metav1.GetOptions{})
	if err != nil {
		t.Errorf("namespace could not be accessed after test failed: %v", err)
		return
	}

	t.Logf("test succeeded. Namespace %s was preserved after subtest failed", "testns")
}
