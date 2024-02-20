package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalFilePersister(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		path         string
		existingData string
		data         string
		truncates    bool
	}{
		{
			name: "just_file",
			path: "test.txt",
			data: "some data",
		},
		{
			name: "with_dir",
			path: "path/test.txt",
			data: "some data",
		},
		{
			name:         "truncates",
			path:         "test.txt",
			data:         "some data",
			truncates:    true,
			existingData: "existing data",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			p := filepath.Join(dir, tt.path)

			// We want to make sure that the persister truncates the existing
			// data and therefore overwrites existing data. This sets up a file
			// with some existing data that should be overwritten.
			if tt.truncates {
				err := os.WriteFile(p, []byte(tt.existingData), 0o600)
				require.NoError(t, err)
			}

			var l LocalFilePersister
			err := l.Persist(context.Background(), p, strings.NewReader(tt.data))
			assert.NoError(t, err)

			i, err := os.Stat(p)
			require.NoError(t, err)
			assert.False(t, i.IsDir())

			bb, err := os.ReadFile(filepath.Clean(p))
			require.NoError(t, err)

			if tt.truncates {
				assert.NotEqual(t, tt.existingData, string(bb))
			}

			assert.Equal(t, tt.data, string(bb))
		})
	}
}

func TestRemoteFilePersister(t *testing.T) {
	t.Parallel()

	basePath := "screenshots"
	presignedEndpoint := "/presigned"
	uploadEndpoint := "/upload"

	tests := []struct {
		name                    string
		path                    string
		dataToUpload            string
		wantPresignedURLBody    string
		wantPresignedHeaders    map[string]string
		uploadResponse          int
		getPresignedURLResponse int
		wantError               string
	}{
		{
			name:         "upload_file",
			path:         "some/path/file.png",
			dataToUpload: "here's some data",
			wantPresignedURLBody: `{
					"service":"aws_s3",
					"operation": "upload",
					"files":[{"name":"screenshots/some/path/file.png"}]
				}`,
			wantPresignedHeaders: map[string]string{
				"Authorization": "token asd123",
				"Run_id":        "123456",
			},
			uploadResponse:          http.StatusOK,
			getPresignedURLResponse: http.StatusOK,
		},
		{
			name:         "get_presigned_rate_limited",
			path:         "some/path/file.png",
			dataToUpload: "here's some data",
			wantPresignedURLBody: `{
					"service":"aws_s3",
					"operation": "upload",
					"files":[{"name":"screenshots/some/path/file.png"}]
				}`,
			wantPresignedHeaders: map[string]string{
				"Authorization": "token asd123",
				"Run_id":        "123456",
			},
			getPresignedURLResponse: http.StatusTooManyRequests,
			wantError:               "getting presigned url: server returned 429 (too many requests)",
		},
		{
			name:         "get_presigned_fails",
			path:         "some/path/file.png",
			dataToUpload: "here's some data",
			wantPresignedURLBody: `{
					"service":"aws_s3",
					"operation": "upload",
					"files":[{"name":"screenshots/some/path/file.png"}]
				}`,
			wantPresignedHeaders: map[string]string{
				"Authorization": "token asd123",
				"Run_id":        "123456",
			},
			getPresignedURLResponse: http.StatusInternalServerError,
			wantError:               "getting presigned url: server returned 500 (internal server error)",
		},
		{
			name:         "upload_rate_limited",
			path:         "some/path/file.png",
			dataToUpload: "here's some data",
			wantPresignedURLBody: `{
					"service":"aws_s3",
					"operation": "upload",
					"files":[{"name":"screenshots/some/path/file.png"}]
				}`,
			wantPresignedHeaders: map[string]string{
				"Authorization": "token asd123",
				"Run_id":        "123456",
			},
			uploadResponse:          http.StatusTooManyRequests,
			getPresignedURLResponse: http.StatusOK,
			wantError:               "uploading: server returned 429 (too many requests)",
		},
		{
			name:         "upload_fails",
			path:         "some/path/file.png",
			dataToUpload: "here's some data",
			wantPresignedURLBody: `{
					"service":"aws_s3",
					"operation": "upload",
					"files":[{"name":"screenshots/some/path/file.png"}]
				}`,
			wantPresignedHeaders: map[string]string{
				"Authorization": "token asd123",
				"Run_id":        "123456",
			},
			uploadResponse:          http.StatusInternalServerError,
			getPresignedURLResponse: http.StatusOK,
			wantError:               "uploading: server returned 500 (internal server error)",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mux := http.NewServeMux()
			s := httptest.NewServer(mux)
			defer s.Close()

			// This handles the request to retrieve a presigned url.
			mux.HandleFunc(presignedEndpoint, http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					defer r.Body.Close() //nolint:errcheck

					bb, err := io.ReadAll(r.Body)
					require.NoError(t, err)

					// Ensures that the body of the request matches the
					// expected format.
					assert.JSONEq(t, tt.wantPresignedURLBody, string(bb))

					// Ensures that the headers are sent to the server from
					// the browser module.
					for k, v := range tt.wantPresignedHeaders {
						assert.Equal(t, v, r.Header[k][0])
					}

					w.WriteHeader(tt.getPresignedURLResponse)
					_, err = fmt.Fprintf(w, `{
							"service": "aws_s3",
							"urls": [{
								"name": "%s",
								"pre_signed_url": "%s"
							}]
							}`, basePath, s.URL+uploadEndpoint)

					require.NoError(t, err)
				},
			))

			// This handles the upload of the files with the presigned url that
			// is retrieved from the handler above.
			mux.HandleFunc(uploadEndpoint, http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					defer r.Body.Close() //nolint:errcheck

					bb, err := io.ReadAll(r.Body)
					require.NoError(t, err)

					// Ensures that the data is uploaded to the server and matches
					// what was sent.
					assert.Equal(t, tt.dataToUpload, string(bb))

					w.WriteHeader(tt.uploadResponse)
				}))

			r := NewRemoteFilePersister(s.URL+presignedEndpoint, tt.wantPresignedHeaders, basePath)
			err := r.Persist(context.Background(), tt.path, strings.NewReader(tt.dataToUpload))
			if tt.wantError != "" {
				assert.EqualError(t, err, tt.wantError)
				return
			}

			assert.NoError(t, err)
		})
	}
}
