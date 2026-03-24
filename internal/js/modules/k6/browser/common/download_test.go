package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownload(t *testing.T) {
	t.Parallel()

	t.Run("accessors", func(t *testing.T) {
		t.Parallel()

		dl := newDownload(nil, "guid-123", "https://example.com/file.zip", "file.zip", "/tmp/downloads")

		assert.Equal(t, "https://example.com/file.zip", dl.URL())
		assert.Equal(t, "file.zip", dl.SuggestedFilename())
	})

	t.Run("page_returns_initiating_page", func(t *testing.T) {
		t.Parallel()

		page := &Page{}
		dl := newDownload(page, "guid-123", "https://example.com/file.zip", "file.zip", "/tmp/downloads")

		assert.Equal(t, page, dl.Page())
	})

	t.Run("page_returns_nil_when_no_page", func(t *testing.T) {
		t.Parallel()

		dl := newDownload(nil, "guid-123", "https://example.com/file.zip", "file.zip", "/tmp/downloads")

		assert.Nil(t, dl.Page())
	})

	t.Run("path_after_completion", func(t *testing.T) {
		t.Parallel()

		dl := newDownload(nil, "guid-123", "https://example.com/file.zip", "file.zip", "/tmp/downloads")
		dl.finish("")

		path, err := dl.Path()
		require.NoError(t, err)
		assert.Equal(t, "/tmp/downloads/guid-123", path)
	})

	t.Run("path_after_cancel", func(t *testing.T) {
		t.Parallel()

		dl := newDownload(nil, "guid-123", "https://example.com/file.zip", "file.zip", "/tmp/downloads")
		dl.finish("canceled")

		_, err := dl.Path()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "canceled")
	})

	t.Run("failure_returns_empty_on_success", func(t *testing.T) {
		t.Parallel()

		dl := newDownload(nil, "guid-123", "https://example.com/file.zip", "file.zip", "/tmp/downloads")
		dl.finish("")

		assert.Empty(t, dl.Failure())
	})

	t.Run("failure_returns_error_on_cancel", func(t *testing.T) {
		t.Parallel()

		dl := newDownload(nil, "guid-123", "https://example.com/file.zip", "file.zip", "/tmp/downloads")
		dl.finish("canceled")

		assert.Equal(t, "canceled", dl.Failure())
	})

	t.Run("cancel_after_completion_is_noop", func(t *testing.T) {
		t.Parallel()

		dl := newDownload(nil, "guid-123", "https://example.com/file.zip", "file.zip", "/tmp/downloads")
		dl.finish("")
		require.NoError(t, dl.Cancel())

		assert.Empty(t, dl.Failure())
	})

	t.Run("finish_only_once", func(t *testing.T) {
		t.Parallel()

		dl := newDownload(nil, "guid-123", "https://example.com/file.zip", "file.zip", "/tmp/downloads")
		dl.finish("")
		dl.finish("canceled") // second call should be ignored

		assert.Empty(t, dl.Failure())
	})

	t.Run("save_as", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		srcDir := filepath.Join(dir, "src")
		require.NoError(t, os.MkdirAll(srcDir, 0o750))

		// Create a fake downloaded file.
		content := []byte("file content")
		require.NoError(t, os.WriteFile(filepath.Join(srcDir, "guid-123"), content, 0o600))

		dl := newDownload(nil, "guid-123", "https://example.com/file.zip", "file.zip", srcDir)
		dl.finish("")

		dst := filepath.Join(dir, "dst", "saved.zip")
		require.NoError(t, dl.SaveAs(dst))

		got, err := os.ReadFile(dst)
		require.NoError(t, err)
		assert.Equal(t, content, got)
	})

	t.Run("save_as_fails_on_canceled", func(t *testing.T) {
		t.Parallel()

		dl := newDownload(nil, "guid-123", "https://example.com/file.zip", "file.zip", "/tmp/downloads")
		dl.finish("canceled")

		err := dl.SaveAs("/tmp/out.zip")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "canceled")
	})
}
