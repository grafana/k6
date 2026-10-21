package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeUploader struct {
	mu      sync.Mutex
	release chan struct{} // if set, each upload waits for it
	err     error
	paths   []string
	data    []string
}

func (f *fakeUploader) Upload(_ context.Context, path string, _ time.Time, data io.Reader) (string, error) {
	if f.release != nil {
		<-f.release
	}
	bb, err := io.ReadAll(data)
	if err != nil {
		return "", err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.paths = append(f.paths, path)
	f.data = append(f.data, string(bb))
	if f.err != nil {
		return "", f.err
	}

	return "uuid-" + path, nil
}

func (f *fakeUploader) uploaded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.paths...)
}

func TestAsyncPersister(t *testing.T) {
	t.Parallel()

	t.Run("uploads_in_order_without_blocking", func(t *testing.T) {
		t.Parallel()

		u := &fakeUploader{release: make(chan struct{})}
		logger, hook := logtest.NewNullLogger()
		a := NewAsyncPersister(u, logger, 10, time.Second)

		var capturedAt []time.Time
		for _, p := range []string{"a.png", "b.png", "c.png"} {
			capturedAt = append(capturedAt, time.Now())
			// Persist must return while the upload is blocked.
			require.NoError(t, a.Persist(context.Background(), p, strings.NewReader("data-"+p)))
		}
		assert.Empty(t, u.uploaded())

		close(u.release)
		require.NoError(t, a.Flush(context.Background()))

		assert.Equal(t, []string{"a.png", "b.png", "c.png"}, u.uploaded())
		assert.Equal(t, []string{"data-a.png", "data-b.png", "data-c.png"}, u.data)

		entries := hook.AllEntries()
		require.Len(t, entries, 3)
		for i, p := range []string{"a.png", "b.png", "c.png"} {
			assert.Equal(t, logrus.InfoLevel, entries[i].Level)
			assert.Equal(t, "screenshot:uuid-"+p, entries[i].Message)
			// The log entry carries the capture time, not the upload time.
			assert.WithinDuration(t, capturedAt[i], entries[i].Time, 50*time.Millisecond)
		}
	})

	t.Run("copies_data", func(t *testing.T) {
		t.Parallel()

		u := &fakeUploader{release: make(chan struct{})}
		logger, _ := logtest.NewNullLogger()
		a := NewAsyncPersister(u, logger, 10, time.Second)

		buf := []byte("original")
		require.NoError(t, a.Persist(context.Background(), "a.png", bytes.NewReader(buf)))
		copy(buf, "changed!")

		close(u.release)
		require.NoError(t, a.Flush(context.Background()))
		assert.Equal(t, []string{"original"}, u.data)
	})

	t.Run("drops_when_queue_is_full", func(t *testing.T) {
		t.Parallel()

		u := &fakeUploader{release: make(chan struct{})}
		logger, hook := logtest.NewNullLogger()
		a := NewAsyncPersister(u, logger, 1, time.Second)

		// The first file is taken by the worker, which then blocks, the second
		// fills the queue and the third is dropped.
		require.NoError(t, a.Persist(context.Background(), "a.png", strings.NewReader("a")))
		require.Eventually(t, func() bool { return len(a.queue) == 0 }, time.Second, time.Millisecond)
		require.NoError(t, a.Persist(context.Background(), "b.png", strings.NewReader("b")))
		require.NoError(t, a.Persist(context.Background(), "c.png", strings.NewReader("c")))

		close(u.release)
		require.NoError(t, a.Flush(context.Background()))

		assert.Equal(t, []string{"a.png", "b.png"}, u.uploaded())
		assert.Contains(t, messages(hook, logrus.WarnLevel), `dropping screenshot "c.png": upload queue is full`)
	})

	t.Run("drops_after_flush", func(t *testing.T) {
		t.Parallel()

		u := &fakeUploader{}
		logger, hook := logtest.NewNullLogger()
		a := NewAsyncPersister(u, logger, 10, time.Second)

		require.NoError(t, a.Flush(context.Background()))
		require.NoError(t, a.Persist(context.Background(), "a.png", strings.NewReader("a")))

		assert.Empty(t, u.uploaded())
		assert.Contains(t, messages(hook, logrus.WarnLevel), `dropping screenshot "a.png": uploads have stopped`)
	})

	t.Run("logs_upload_errors", func(t *testing.T) {
		t.Parallel()

		u := &fakeUploader{err: errors.New("server returned 429 (too many requests)")}
		logger, hook := logtest.NewNullLogger()
		a := NewAsyncPersister(u, logger, 10, time.Second)

		require.NoError(t, a.Persist(context.Background(), "a.png", strings.NewReader("a")))
		require.NoError(t, a.Flush(context.Background()))

		assert.Equal(t,
			[]string{`uploading screenshot "a.png": server returned 429 (too many requests)`},
			messages(hook, logrus.WarnLevel))
		assert.Empty(t, messages(hook, logrus.InfoLevel))
	})

	t.Run("flush_times_out", func(t *testing.T) {
		t.Parallel()

		u := &fakeUploader{release: make(chan struct{})}
		defer close(u.release)
		logger, _ := logtest.NewNullLogger()
		a := NewAsyncPersister(u, logger, 10, time.Second)

		require.NoError(t, a.Persist(context.Background(), "a.png", strings.NewReader("a")))
		require.NoError(t, a.Persist(context.Background(), "b.png", strings.NewReader("b")))

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		err := a.Flush(ctx)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}

func messages(hook *logtest.Hook, level logrus.Level) []string {
	var msgs []string
	for _, e := range hook.AllEntries() {
		if e.Level == level {
			msgs = append(msgs, e.Message)
		}
	}

	return msgs
}
