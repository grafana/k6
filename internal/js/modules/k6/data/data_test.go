package data

import (
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
		slots: make(map[string]*sharedArraySlot),
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
