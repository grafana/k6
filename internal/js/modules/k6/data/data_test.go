package data

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/js/modulestest"
)

func TestNewSharedArrayFromReusesExistingArrays(t *testing.T) {
	t.Parallel()

	runtime := modulestest.NewRuntime(t)
	dataModule, ok := New().NewModuleInstance(runtime.VU).(*Data)
	require.True(t, ok)

	reader := newCountingRecordReader([][]string{
		{"a", "b"},
	})

	dataModule.NewSharedArrayFrom(runtime.VU.Runtime(), "shared", reader)
	require.Positive(t, reader.reads.Load())

	anotherReader := newCountingRecordReader([][]string{
		{"x", "y"},
	})

	dataModule.NewSharedArrayFrom(runtime.VU.Runtime(), "shared", anotherReader)
	require.Zero(t, anotherReader.reads.Load())
}

func TestSharedArraysLoadOrStoreBuildsOnce(t *testing.T) {
	t.Parallel()

	arrays := &sharedArrays{
		data: make(map[string]sharedArray),
	}

	var buildsCount atomic.Int32
	builder := func() sharedArray {
		buildsCount.Add(1)
		return sharedArray{arr: []string{"v"}}
	}

	var wg sync.WaitGroup
	const goroutines = 10
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			arrays.loadOrStore("shared", builder)
		}()
	}

	wg.Wait()
	require.Equal(t, int32(1), buildsCount.Load())
}

type countingRecordReader struct {
	records [][]string
	index   int
	reads   atomic.Int32
}

func newCountingRecordReader(records [][]string) *countingRecordReader {
	return &countingRecordReader{records: records}
}

func (r *countingRecordReader) Read() (any, error) {
	if r.index >= len(r.records) {
		return nil, io.EOF
	}

	r.reads.Add(1)
	record := r.records[r.index]
	r.index++
	return record, nil
}

func TestNewSharedArrayFromConcurrentMultiVU(t *testing.T) {
	t.Parallel()

	root := New()
	var totalReads atomic.Int32

	var wg sync.WaitGroup
	const vus = 10

	for i := 0; i < vus; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runtime := modulestest.NewRuntime(t)
			dataModule, ok := root.NewModuleInstance(runtime.VU).(*Data)
			require.True(t, ok)

			reader := &sharedCountingRecordReader{
				records: [][]string{{"a", "b"}, {"c", "d"}},
				reads:   &totalReads,
			}
			dataModule.NewSharedArrayFrom(runtime.VU.Runtime(), "concurrent-test", reader)
		}()
	}

	wg.Wait()
	// Only one reader should have been consumed (2 records read once)
	require.Equal(t, int32(2), totalReads.Load())
}

// sharedCountingRecordReader uses a shared atomic counter across instances.
type sharedCountingRecordReader struct {
	records [][]string
	index   int
	reads   *atomic.Int32
}

func (r *sharedCountingRecordReader) Read() (any, error) {
	if r.index >= len(r.records) {
		return nil, io.EOF
	}

	r.reads.Add(1)
	record := r.records[r.index]
	r.index++
	return record, nil
}

func TestNewSharedArrayFromReaderError(t *testing.T) {
	t.Parallel()

	runtime := modulestest.NewRuntime(t)
	dataModule, ok := New().NewModuleInstance(runtime.VU).(*Data)
	require.True(t, ok)

	errReader := &errorRecordReader{
		records: [][]string{{"a", "b"}},
		errAt:   1, // Error on second read
	}

	require.Panics(t, func() {
		dataModule.NewSharedArrayFrom(runtime.VU.Runtime(), "error-test", errReader)
	})
}

type errorRecordReader struct {
	records [][]string
	index   int
	errAt   int
}

func (r *errorRecordReader) Read() (any, error) {
	if r.index == r.errAt {
		return nil, errors.New("simulated read error")
	}
	if r.index >= len(r.records) {
		return nil, io.EOF
	}
	record := r.records[r.index]
	r.index++
	return record, nil
}

func TestNewSharedArrayFromMarshalError(t *testing.T) {
	t.Parallel()

	runtime := modulestest.NewRuntime(t)
	dataModule, ok := New().NewModuleInstance(runtime.VU).(*Data)
	require.True(t, ok)

	// channels cannot be marshaled to JSON
	unmarshalableReader := &unmarshalableRecordReader{}

	require.Panics(t, func() {
		dataModule.NewSharedArrayFrom(runtime.VU.Runtime(), "marshal-error", unmarshalableReader)
	})
}

type unmarshalableRecordReader struct {
	called bool
}

func (r *unmarshalableRecordReader) Read() (any, error) {
	if r.called {
		return nil, io.EOF
	}
	r.called = true
	// channels can't be JSON marshaled
	return make(chan int), nil
}

func TestNewSharedArrayFromEmpty(t *testing.T) {
	t.Parallel()

	runtime := modulestest.NewRuntime(t)
	dataModule, ok := New().NewModuleInstance(runtime.VU).(*Data)
	require.True(t, ok)

	emptyReader := newCountingRecordReader([][]string{})

	result := dataModule.NewSharedArrayFrom(runtime.VU.Runtime(), "empty", emptyReader)
	require.NotNil(t, result)

	length := result.Get("length")
	require.Equal(t, int64(0), length.ToInteger())
}

func TestNewSharedArrayFromDifferentNames(t *testing.T) {
	t.Parallel()

	runtime := modulestest.NewRuntime(t)
	dataModule, ok := New().NewModuleInstance(runtime.VU).(*Data)
	require.True(t, ok)

	reader1 := newCountingRecordReader([][]string{{"a"}})
	reader2 := newCountingRecordReader([][]string{{"b"}})

	dataModule.NewSharedArrayFrom(runtime.VU.Runtime(), "name1", reader1)
	dataModule.NewSharedArrayFrom(runtime.VU.Runtime(), "name2", reader2)

	// Both readers should have been consumed since names are different
	require.Positive(t, reader1.reads.Load())
	require.Positive(t, reader2.reads.Load())
}
