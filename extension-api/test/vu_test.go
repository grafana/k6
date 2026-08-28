package extensionapitest

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"

	extensionapi "go.k6.io/k6-extension-api"
)

func TestVURunsPromiseCallbacks(t *testing.T) {
	vu := NewVU()
	promise, resolver := vu.NewPromise()
	if err := vu.Runtime().Set("promise", promise); err != nil {
		t.Fatal(err)
	}

	go resolver.Resolve(42)
	if err := vu.Run(func() error {
		_, err := vu.Runtime().RunString(`promise.then((value) => { globalThis.result = value })`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got := vu.Runtime().Get("result").ToInteger(); got != 42 {
		t.Fatalf("unexpected promise result %d", got)
	}
}

func TestVURunsSynchronouslySettledPromise(t *testing.T) {
	vu := NewVU()
	promise, resolver := vu.NewPromise()
	if err := vu.Runtime().Set("promise", promise); err != nil {
		t.Fatal(err)
	}

	if err := vu.Run(func() error {
		_, err := vu.Runtime().RunString(`promise.then((value) => { globalThis.result = value })`)
		resolver.Resolve(42)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got := vu.Runtime().Get("result").ToInteger(); got != 42 {
		t.Fatalf("unexpected promise result %d", got)
	}
}

func TestVUReportsUnhandledPromiseRejections(t *testing.T) {
	vu := NewVU()
	promise, resolver := vu.NewPromise()
	if err := vu.Runtime().Set("promise", promise); err != nil {
		t.Fatal(err)
	}

	go resolver.Reject("failed")
	if err := vu.Run(func() error {
		_, err := vu.Runtime().RunString(`promise`)
		return err
	}); err == nil {
		t.Fatal("expected an unhandled promise rejection")
	}
}

func TestVUMetrics(t *testing.T) {
	vu := NewVU()
	vu.SetCurrentTags(extensionapi.NewTags(map[string]string{"scenario": "test"}, nil))
	vu.EnabledSystemTag[extensionapi.SystemTagURL] = true
	metric, err := vu.RegisterMetric(extensionapi.MetricSpec{Name: "example", Kind: extensionapi.MetricCounter})
	if err != nil {
		t.Fatal(err)
	}

	tags := vu.WithSystemTags(vu.CurrentTags(), map[extensionapi.SystemTag]string{
		extensionapi.SystemTagURL: "https://example.test",
	})
	if err := vu.Emit(context.Background(), []extensionapi.Sample{{Metric: metric, Value: 1, Tags: tags}}); err != nil {
		t.Fatal(err)
	}

	samples := vu.Samples()
	if len(samples) != 1 {
		t.Fatalf("got %d samples, want 1", len(samples))
	}
	if got := samples[0].Tags.Values()["scenario"]; got != "test" {
		t.Fatalf("unexpected scenario tag %q", got)
	}
	if got := samples[0].Tags.Values()["url"]; got != "https://example.test" {
		t.Fatalf("unexpected URL tag %q", got)
	}
}

func TestVUWithSystemTagsHandlesEmptyTags(t *testing.T) {
	t.Parallel()
	vu := NewVU()
	vu.EnabledSystemTag[extensionapi.SystemTagURL] = true

	tags := vu.WithSystemTags(NewTags(nil, nil), map[extensionapi.SystemTag]string{
		extensionapi.SystemTagURL: "https://example.test",
	})
	if got := tags.Values()["url"]; got != "https://example.test" {
		t.Fatalf("URL tag = %q", got)
	}
}

func TestVUFileSystem(t *testing.T) {
	t.Parallel()
	vu := NewVU()
	_, err := vu.FileSystem()
	if err != extensionapi.ErrFileSystemUnavailable {
		t.Fatalf("FileSystem error = %v, want %v", err, extensionapi.ErrFileSystemUnavailable)
	}

	vu.FileSystemValue = fstest.MapFS{"keystore.jks": {Data: []byte("key")}}
	fileSystem, err := vu.FileSystem()
	if err != nil {
		t.Fatal(err)
	}
	data, err := fs.ReadFile(fileSystem, "keystore.jks")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "key" {
		t.Fatalf("file contents = %q, want %q", data, "key")
	}
}

func TestVUMapsStructFieldsToSnakeCase(t *testing.T) {
	vu := NewVU()
	if err := vu.Runtime().Set("value", struct {
		CommonName string
		Ignored    string `js:"-"`
	}{CommonName: "example", Ignored: "ignored"}); err != nil {
		t.Fatal(err)
	}

	result, err := vu.Runtime().RunString(`value.common_name + ":" + value.ignored`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.String(); got != "example:undefined" {
		t.Fatalf("unexpected mapped fields %q", got)
	}
}
