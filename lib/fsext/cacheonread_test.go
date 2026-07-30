package fsext

import (
	"io"
	"sync"
	"syscall"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// TestCacheOnReadFsConcurrentFirstOpen exercises the race where two goroutines
// open the same not-yet-cached file at the same time. Without per-path
// serialization in CacheOnReadFs the underlying afero CacheOnReadFs can be
// caught mid copyToLayer by a second reader, which then opens the layer file
// while it is still being filled and reads zero or partial bytes.
func TestCacheOnReadFsConcurrentFirstOpen(t *testing.T) {
	t.Parallel()
	assertConcurrentFirstOpenReadsFullContent(t, func(fs afero.Fs, path string) (afero.File, error) {
		return fs.Open(path)
	})
}

// TestCacheOnReadFsConcurrentFirstOpenFile mirrors the plain Open test but
// exercises the OpenFile entry point, which shares openSerialized. Guards
// against a future refactor reintroducing the race on that path.
func TestCacheOnReadFsConcurrentFirstOpenFile(t *testing.T) {
	t.Parallel()
	assertConcurrentFirstOpenReadsFullContent(t, func(fs afero.Fs, path string) (afero.File, error) {
		return fs.OpenFile(path, syscall.O_RDONLY, 0)
	})
}

// assertConcurrentFirstOpenReadsFullContent releases N goroutines simultaneously
// at the first-open of a not-yet-cached path and asserts every goroutine reads
// the full file content. The opener parameter picks the entry point under test
// (Open vs OpenFile).
func assertConcurrentFirstOpenReadsFullContent(t *testing.T, opener func(afero.Fs, string) (afero.File, error)) {
	t.Helper()

	const (
		path        = "/data.csv"
		concurrency = 32
	)
	content := []byte("col1,col2\na,b\nc,d\n")

	base := NewMemMapFs()
	require.NoError(t, WriteFile(base, path, content, 0o644))

	fs := NewCacheOnReadFs(base, NewMemMapFs(), 0)

	var (
		wg    sync.WaitGroup
		start = make(chan struct{})
	)
	results := make([][]byte, concurrency)
	errs := make([]error, concurrency)

	for i := range concurrency {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			f, err := opener(fs, path)
			if err != nil {
				errs[idx] = err
				return
			}
			defer f.Close() //nolint:errcheck
			results[idx], errs[idx] = io.ReadAll(f)
		}(i)
	}

	close(start)
	wg.Wait()

	for i, got := range results {
		require.NoError(t, errs[i], "goroutine %d", i)
		require.Equal(t, content, got, "goroutine %d read truncated data", i)
	}
}
