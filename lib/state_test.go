package lib

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTagMapSet(t *testing.T) {
	t.Parallel()

	t.Run("Sync", func(t *testing.T) {
		t.Parallel()

		tm := NewTagMap(nil)
		tm.Set("mytag", "42")
		v, found := tm.Get("mytag")
		assert.True(t, found)
		assert.Equal(t, "42", v)
	})

	t.Run("Safe-Concurrent", func(t *testing.T) {
		t.Parallel()
		tm := NewTagMap(nil)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			count := 0
			for {
				select {
				case <-time.Tick(1 * time.Millisecond):
					count++
					tm.Set("mytag", strconv.Itoa(count))

				case <-ctx.Done():
					return
				}
			}
		}()

		go func() {
			for {
				select {
				case <-time.Tick(1 * time.Millisecond):
					tm.Get("mytag")

				case <-ctx.Done():
					return
				}
			}
		}()

		time.Sleep(100 * time.Millisecond)
	})
}

func TestTagMapGet(t *testing.T) {
	t.Parallel()
	tm := NewTagMap(map[string]string{
		"key1": "value1",
	})
	v, ok := tm.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", v)
}

func TestTagMapLen(t *testing.T) {
	t.Parallel()
	tm := NewTagMap(map[string]string{
		"key1": "value1",
		"key2": "value2",
	})
	assert.Equal(t, 2, tm.Len())
}

func TestTagMapDelete(t *testing.T) {
	t.Parallel()
	m := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	tm := NewTagMap(m)
	tm.Delete("key1")
	_, ok := m["key1"]
	assert.False(t, ok)
}

func TestTagMapClone(t *testing.T) {
	t.Parallel()
	tm := NewTagMap(map[string]string{
		"key1": "value1",
		"key2": "value2",
	})
	m := tm.Clone()
	assert.Equal(t, map[string]string{
		"key1": "value1",
		"key2": "value2",
	}, m)
}
