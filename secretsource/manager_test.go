package secretsource

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mapSource map[string]string

func (m mapSource) Description() string { return "map source" }

func (m mapSource) Get(key string) (string, error) {
	v, ok := m[key]
	if !ok {
		return "", assert.AnError
	}
	return v, nil
}

// blockingSource lets tests hold a Get call open so concurrent callers can
// pile up on the same key.
type blockingSource struct {
	value   string
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockingSource) Description() string { return "blocking source" }

func (b *blockingSource) Get(_ string) (string, error) {
	b.once.Do(func() { close(b.started) })
	<-b.release
	return b.value, nil
}

func TestManagerGetRedactsBeforeReturning(t *testing.T) {
	t.Parallel()

	const secret = "concurrent-secret-value"
	mgr, hook, err := NewManager(map[string]Source{
		"default": mapSource{"k": secret},
	})
	require.NoError(t, err)

	got, err := mgr.Get(DefaultSourceName, "k")
	require.NoError(t, err)
	require.Equal(t, secret, got)

	entry := &logrus.Entry{
		Message: "leaked " + secret + " end",
		Data:    logrus.Fields{},
	}
	require.NoError(t, hook.Fire(entry))
	assert.NotContains(t, entry.Message, secret)
	assert.Contains(t, entry.Message, "***SECRET_REDACTED***")
}

// TestManagerGetConcurrentFirstFetchRedacts locks the Store-before-add race:
// secrets.get() runs Manager.Get in a goroutine per call (and across VUs), so
// two first-time fetches of the same key can interleave. Caching the value
// before registering it with the redaction hook lets the second caller return
// and log the plaintext secret before redaction is armed — including into the
// cloud log pusher for k6 cloud run --local-execution.
func TestManagerGetConcurrentFirstFetchRedacts(t *testing.T) {
	t.Parallel()

	const (
		secret  = "race-secret-value-do-not-leak"
		workers = 32
	)

	src := &blockingSource{
		value:   secret,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	mgr, hook, err := NewManager(map[string]Source{
		"default": src,
	})
	require.NoError(t, err)

	var (
		wg     sync.WaitGroup
		leaked atomic.Int64
		start  = make(chan struct{})
	)

	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			<-start
			got, err := mgr.Get(DefaultSourceName, "k")
			if err != nil {
				return
			}
			// Log immediately on return — this is the window a VU / cloud log
			// hook would observe the secret if redaction was not yet armed.
			entry := &logrus.Entry{
				Message: "value=" + got,
				Data:    logrus.Fields{"secret": got},
			}
			_ = hook.Fire(entry)
			if strings.Contains(entry.Message, secret) || strings.Contains(entry.Data["secret"].(string), secret) {
				leaked.Add(1)
			}
		}()
	}

	close(start)
	// Wait until the first source.Get is in flight, then let many Get callers
	// contend on the cache/redaction path as that fetch completes.
	<-src.started
	close(src.release)
	wg.Wait()

	assert.Equal(t, int64(0), leaked.Load(),
		"concurrent Get must not return a secret before the redaction hook knows about it")
}
