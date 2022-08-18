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
	cv := tm.GetCurrentValues()
	v, found := cv.Tags.Get("mytag")
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

	stateTags := NewVUStateTags(metrics.NewRegistry().RootTagSet())
	go func() {
		defer wg.Done()
		count := 0
		for {
			select {
			case <-time.Tick(1 * time.Millisecond):
				count++
				stateTags.Modify(func(tm *metrics.TagsAndMeta) {
					val := strconv.Itoa(count)
					tm.SetMetadata("mymeta", val)
					tm.SetTag("mytag", val)
				})

			case <-ctx.Done():
				exp := strconv.Itoa(count)
				cv := stateTags.GetCurrentValues()
				val, ok := cv.Tags.Get("mytag")
				assert.True(t, ok)
				assert.Equal(t, exp, val)

				metaval, metaok := cv.Metadata["mymeta"]
				assert.True(t, metaok)
				assert.Equal(t, exp, metaval)
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for {
			select {
			case <-time.Tick(1 * time.Millisecond):
				cv := stateTags.GetCurrentValues()
				cv.Tags.Get("mytag")
				cv.SetMetadata("mymeta", "foo") // just to ensure this won't have any effect

			case <-ctx.Done():
				return
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)
}

func TestVUStateTagsDelete(t *testing.T) {
	t.Parallel()
	stateTags := NewVUStateTags(metrics.NewRegistry().RootTagSet().WithTagsFromMap(map[string]string{
		"key1": "value1",
		"key2": "value2",
	}))

	val, ok := stateTags.GetCurrentValues().Tags.Get("key1")
	require.True(t, ok)
	require.Equal(t, "value1", val)

	stateTags.Modify(func(tam *metrics.TagsAndMeta) {
		tam.DeleteTag("key1")
	})
	_, ok = stateTags.GetCurrentValues().Tags.Get("key1")
	assert.False(t, ok)

	assert.Equal(t, map[string]string{"key2": "value2"}, stateTags.GetCurrentValues().Tags.Map())
}

func TestVUStateTagsMap(t *testing.T) {
	t.Parallel()
	tm := NewVUStateTags(metrics.NewRegistry().RootTagSet().WithTagsFromMap(map[string]string{
		"key1": "value1",
		"key2": "value2",
	}))
	m := tm.GetCurrentValues().Tags.Map()
	assert.Equal(t, map[string]string{
		"key1": "value1",
		"key2": "value2",
	}, m)
}
