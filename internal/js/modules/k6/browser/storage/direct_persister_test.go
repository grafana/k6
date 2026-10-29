package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectFilePersister(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		before := time.Now().UTC()
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/upload", r.URL.Path)
			assert.Equal(t, "Basic abc==", r.Header.Get("Authorization"))

			require.NoError(t, r.ParseMultipartForm(1<<20))
			assert.Equal(t, "exec-1", r.FormValue("execution_id"))

			capturedAt, err := time.Parse(time.RFC3339Nano, r.FormValue("captured_at"))
			require.NoError(t, err)
			assert.False(t, capturedAt.Before(before))

			file, header, err := r.FormFile("file")
			require.NoError(t, err)
			defer file.Close() //nolint:errcheck
			cd := header.Header.Get("Content-Disposition")
			assert.Equal(t, `form-data; name="file"; filename="screenshots/a.png"`, cd)
			bb, err := io.ReadAll(file)
			require.NoError(t, err)
			assert.Equal(t, "image data", string(bb))

			w.WriteHeader(http.StatusCreated)
		}))
		defer s.Close()

		d := NewDirectFilePersister(
			s.URL+"/upload",
			map[string]string{"Authorization": "Basic abc=="},
			map[string]string{"execution_id": "exec-1"},
			"screenshots",
			nil,
		)
		err := d.Persist(context.Background(), "a.png", strings.NewReader("image data"))
		require.NoError(t, err)
	})

	t.Run("status_error", func(t *testing.T) {
		t.Parallel()

		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer s.Close()

		d := NewDirectFilePersister(s.URL, nil, nil, "", nil)
		err := d.Persist(context.Background(), "a.png", strings.NewReader("image data"))
		assert.EqualError(t, err, "uploading: server returned 429 (too many requests)")
	})

	t.Run("request_error_does_not_leak_url", func(t *testing.T) {
		t.Parallel()

		s := httptest.NewServer(http.NotFoundHandler())
		u := s.URL
		s.Close()

		d := NewDirectFilePersister(u+"/upload?token=s3cr3t", nil, nil, "", nil)
		err := d.Persist(context.Background(), "a.png", strings.NewReader("image data"))
		assert.EqualError(t, err, "performing upload request: connecting to host failed")
	})
}
