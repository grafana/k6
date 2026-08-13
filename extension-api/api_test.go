package extensionapi

import (
	"testing"
)

func TestRegister(t *testing.T) {
	t.Parallel()

	name := "k6/x/extension-api-test-register"
	value := &struct{}{}
	Register(name, value)

	registered := Registered()
	if registered[name] != value {
		t.Fatal("registered module is not preserved")
	}

	registered[name] = nil
	if Registered()[name] != value {
		t.Fatal("Registered returned the mutable registry")
	}
}

func TestRegisterRejectsInvalidName(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("Register did not panic")
		}
	}()

	Register("example", struct{}{})
}

func TestTagsAreImmutableSnapshots(t *testing.T) {
	t.Parallel()

	values := map[string]string{"scenario": "first"}
	metadata := map[string]string{"request_id": "one"}
	tags := NewTags(values, metadata)
	values["scenario"] = "changed"
	metadata["request_id"] = "changed"

	if got := tags.Values()["scenario"]; got != "first" {
		t.Fatalf("unexpected tag value %q", got)
	}
	if got := tags.Metadata()["request_id"]; got != "one" {
		t.Fatalf("unexpected metadata value %q", got)
	}

	derived := tags.With(map[string]string{"operation": "write"}).WithMetadata(map[string]string{"attempt": "1"})
	derivedValues := derived.Values()
	derivedValues["operation"] = "changed"
	if got := derived.Values()["operation"]; got != "write" {
		t.Fatalf("returned tag map mutated snapshot: %q", got)
	}
	if _, exists := tags.Values()["operation"]; exists {
		t.Fatal("derived tags changed the original snapshot")
	}

	empty := NewTags(nil, nil).With(map[string]string{"method": "GET"}).WithMetadata(map[string]string{"id": "two"})
	if got := empty.Values()["method"]; got != "GET" {
		t.Fatalf("unexpected tag added to empty snapshot %q", got)
	}
	if got := empty.Metadata()["id"]; got != "two" {
		t.Fatalf("unexpected metadata added to empty snapshot %q", got)
	}
}

func TestMetricFromName(t *testing.T) {
	t.Parallel()

	if got := MetricFromName("extension_api_metric").Name(); got != "extension_api_metric" {
		t.Fatalf("unexpected metric name %q", got)
	}
}
