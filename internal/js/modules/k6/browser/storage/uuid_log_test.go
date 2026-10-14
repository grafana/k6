package storage

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testUUID = "01a13b2d-843f-7e2c-ad24-b2feb8ca004c"

func TestRemoteFilePersisterLogsUUID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		uuidField    string
		uploadStatus int
		wantErr      bool
		wantLogs     []string
	}{
		{
			name:         "uuid_returned",
			uuidField:    fmt.Sprintf(`"uuid":%q,`, testUUID),
			uploadStatus: http.StatusOK,
			wantLogs:     []string{"screenshot:" + testUUID},
		},
		{
			name:         "no_uuid",
			uploadStatus: http.StatusOK,
		},
		{
			name:         "upload_failed",
			uuidField:    fmt.Sprintf(`"uuid":%q,`, testUUID),
			uploadStatus: http.StatusServiceUnavailable,
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mux := http.NewServeMux()
			s := httptest.NewServer(mux)
			defer s.Close()

			mux.HandleFunc("/presign", func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprintf(w,
					`{"service":"aws_s3",%s"urls":[{"name":"a.png","pre_signed_url":%q,"method":"PUT","form_fields":{}}]}`,
					tt.uuidField, s.URL+"/upload")
			})
			mux.HandleFunc("/upload", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.uploadStatus)
			})

			logger, hook := logtest.NewNullLogger()
			r := NewRemoteFilePersister(s.URL+"/presign", nil, "", logger)
			err := r.Persist(context.Background(), "a.png", strings.NewReader("data"))
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assertInfoLogs(t, hook, tt.wantLogs)
		})
	}
}

func TestDirectFilePersisterLogsUUID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   int
		body     string
		wantErr  bool
		wantLogs []string
	}{
		{
			name:     "uuid_returned",
			status:   http.StatusCreated,
			body:     fmt.Sprintf(`{"uuid":%q}`, testUUID),
			wantLogs: []string{"screenshot:" + testUUID},
		},
		{
			name:   "no_uuid",
			status: http.StatusCreated,
			body:   `{}`,
		},
		{
			name:   "non_json_body",
			status: http.StatusOK,
			body:   "ok",
		},
		{
			name:    "upload_failed",
			status:  http.StatusTooManyRequests,
			body:    fmt.Sprintf(`{"uuid":%q}`, testUUID),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer s.Close()

			logger, hook := logtest.NewNullLogger()
			d := NewDirectFilePersister(s.URL, nil, nil, "", logger)
			err := d.Persist(context.Background(), "a.png", strings.NewReader("data"))
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assertInfoLogs(t, hook, tt.wantLogs)
		})
	}
}

func assertInfoLogs(t *testing.T, hook *logtest.Hook, want []string) {
	t.Helper()

	var got []string
	for _, e := range hook.AllEntries() {
		assert.Equal(t, logrus.InfoLevel, e.Level)
		got = append(got, e.Message)
	}
	assert.Equal(t, want, got)
}
