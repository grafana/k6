package ext

import (
	"fmt"
	"sync/atomic"
	"testing"

	extensionapi "go.k6.io/k6-extension-api"
)

var extensionAPITestNumber int64 //nolint:gochecknoglobals // registrations are process-global

type extensionAPITestModule struct{}

func TestGetIncludesExtensionAPIRegistrations(t *testing.T) {
	t.Parallel()

	name := fmt.Sprintf("k6/x/ext-api-%d", atomic.AddInt64(&extensionAPITestNumber, 1))
	module := &extensionAPITestModule{}
	extensionapi.Register(name, module)

	got, ok := Get(JSExtension)[name]
	if !ok {
		t.Fatalf("extension %q was not returned", name)
	}
	if got.Module != module {
		t.Fatal("extension module does not match the registered module")
	}

	for _, extension := range GetAll() {
		if extension.Name == name {
			return
		}
	}
	t.Fatalf("extension %q was not returned by GetAll", name)
}
