package lib

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/metrics"
)

func TestVUStateTagsSync(t *testing.T) {
	t.Parallel()

	tm := NewVUStateTags(metrics.NewRegistry().RootTagSet().With("mytag", "42"))
	v, found := tm.GetCurrentValues().Get("mytag")
	assert.True(t, found)
	assert.Equal(t, "42", v)
}

func TestVUStateTagsSafeConcurrent(t *testing.T) {
	t.Parallel()

	wg := &sync.WaitGroup{}
	wg.Add(2)
	defer wg.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tm := NewVUStateTags(metrics.NewRegistry().RootTagSet())
	go func() {
		defer wg.Done()
		count := 0
		for {
			select {
			case <-time.Tick(1 * time.Millisecond):
				count++
				tm.Modify(func(currentTags *metrics.TagSet) *metrics.TagSet {
					return currentTags.With("mytag", strconv.Itoa(count))
				})

			case <-ctx.Done():
				val, ok := tm.GetCurrentValues().Get("mytag")
				assert.True(t, ok)
				assert.Equal(t, strconv.Itoa(count), val)
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for {
			select {
			case <-time.Tick(1 * time.Millisecond):
				tm.GetCurrentValues().Get("mytag")

			case <-ctx.Done():
				return
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)
}

func TestVUStateTagsDelete(t *testing.T) {
	t.Parallel()
	tm := NewVUStateTags(metrics.NewRegistry().RootTagSet().SortAndAddTags(map[string]string{
		"key1": "value1",
		"key2": "value2",
	}))

	val, ok := tm.GetCurrentValues().Get("key1")
	require.True(t, ok)
	require.Equal(t, "value1", val)

	tm.Modify(func(currentTags *metrics.TagSet) *metrics.TagSet {
		return currentTags.Without("key1")
	})
	_, ok = tm.GetCurrentValues().Get("key1")
	assert.False(t, ok)

	assert.Equal(t, map[string]string{"key2": "value2"}, tm.GetCurrentValues().Map())
}

func TestVUStateTagsMap(t *testing.T) {
	t.Parallel()
	tm := NewVUStateTags(metrics.NewRegistry().RootTagSet().SortAndAddTags(map[string]string{
		"key1": "value1",
		"key2": "value2",
	}))
	m := tm.GetCurrentValues().Map()
	assert.Equal(t, map[string]string{
		"key1": "value1",
		"key2": "value2",
	}, m)
}
