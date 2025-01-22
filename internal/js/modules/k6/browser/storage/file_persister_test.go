package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
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
				err := os.WriteFile(p, []byte(tt.existingData), 0o600) //nolint:forbidigo
				require.NoError(t, err)
			}

			var l LocalFilePersister
			err := l.Persist(context.Background(), p, strings.NewReader(tt.data))
			assert.NoError(t, err)

			i, err := os.Stat(p) //nolint:forbidigo
			require.NoError(t, err)
			assert.False(t, i.IsDir())

			bb, err := os.ReadFile(filepath.Clean(p)) //nolint:forbidigo
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
	if runtime.GOOS == "windows" {
		t.Skip("not supported on windows for now")
		// actual problem is that the paths used are with reverse slash even when they get to be inside JSON
		// which leads to them being parsed as escepe codes when they shouldn't
	}

	const (
		basePath          = "screenshots"
		presignedEndpoint = "/presigned"
		uploadEndpoint    = "/upload"
	)

	tests := []struct {
		name                    string
		path                    string
		dataToUpload            string
		multipartFormFields     map[string]string
		wantPresignedURLBody    string
		wantPresignedHeaders    map[string]string
		wantPresignedURLMethod  string
		uploadResponse          int
		getPresignedURLResponse int
		wantError               string
	}{
		{
			name:         "upload_file",
			path:         "some/path/file.png",
			dataToUpload: "here's some data",
			multipartFormFields: map[string]string{
				"fooKey": "foo",
				"barKey": "bar",
			},
			wantPresignedURLBody: `{
					"service":"aws_s3",
					"operation": "upload_post",
					"files":[{"name":"%s"}]
				}`,
			wantPresignedHeaders: map[string]string{
				"Authorization": "token asd123",
				"Run_id":        "123456",
			},
			wantPresignedURLMethod:  http.MethodPost,
			uploadResponse:          http.StatusOK,
			getPresignedURLResponse: http.StatusOK,
		},
		{
			name:         "upload_file",
			path:         "some/path/file.png",
			dataToUpload: "here's some data",
			multipartFormFields: map[string]string{ // provide different form fields then the previous test
				"bazKey": "baz",
				"quxKey": "qux",
			},
			wantPresignedURLBody: `{
					"service":"aws_s3",
					"operation": "upload_post",
					"files":[{"name":"%s"}]
				}`,
			wantPresignedHeaders: map[string]string{
				"Authorization": "token asd123",
				"Run_id":        "123456",
			},
			wantPresignedURLMethod:  http.MethodPut, // accepts dynamic methods
			uploadResponse:          http.StatusOK,
			getPresignedURLResponse: http.StatusOK,
		},
		{
			name:         "get_presigned_rate_limited",
			path:         "some/path/file.png",
			dataToUpload: "here's some data",
			wantPresignedURLBody: `{
					"service":"aws_s3",
					"operation": "upload_post",
					"files":[{"name":"%s"}]
				}`,
			wantPresignedHeaders: map[string]string{
				"Authorization": "token asd123",
				"Run_id":        "123456",
			},
			wantPresignedURLMethod:  http.MethodPost,
			getPresignedURLResponse: http.StatusTooManyRequests,
			wantError:               "requesting presigned url: server returned 429 (too many requests)",
		},
		{
			name:         "get_presigned_fails",
			path:         "some/path/file.png",
			dataToUpload: "here's some data",
			wantPresignedURLBody: `{
					"service":"aws_s3",
					"operation": "upload_post",
					"files":[{"name":"%s"}]
				}`,
			wantPresignedHeaders: map[string]string{
				"Authorization": "token asd123",
				"Run_id":        "123456",
			},
			wantPresignedURLMethod:  http.MethodPost,
			getPresignedURLResponse: http.StatusInternalServerError,
			wantError:               "requesting presigned url: server returned 500 (internal server error)",
		},
		{
			name:         "upload_rate_limited",
			path:         "some/path/file.png",
			dataToUpload: "here's some data",
			wantPresignedURLBody: `{
					"service":"aws_s3",
					"operation": "upload_post",
					"files":[{"name":"%s"}]
				}`,
			wantPresignedHeaders: map[string]string{
				"Authorization": "token asd123",
				"Run_id":        "123456",
			},
			wantPresignedURLMethod:  http.MethodPost,
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
					"operation": "upload_post",
					"files":[{"name":"%s"}]
				}`,
			wantPresignedHeaders: map[string]string{
				"Authorization": "token asd123",
				"Run_id":        "123456",
			},
			wantPresignedURLMethod:  http.MethodPost,
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

					// Does the response match the expected format?
					wantPresignedURLBody := fmt.Sprintf(
						tt.wantPresignedURLBody,
						filepath.Join(basePath, tt.path),
					)
					assert.JSONEq(t, wantPresignedURLBody, string(bb))

					// Do the HTTP headers are sent to the server from the browser module?
					for k, v := range tt.wantPresignedHeaders {
						assert.Equal(t, v, r.Header[k][0])
					}

					var formFields string
					for k, v := range tt.multipartFormFields {
						formFields += fmt.Sprintf(`"%s":"%s",`, k, v)
					}
					formFields = strings.TrimRight(formFields, ",")

					w.WriteHeader(tt.getPresignedURLResponse)
					_, err = fmt.Fprintf(w, `{
							"service": "aws_s3",
							"urls": [{
								"name": "%s",
								"pre_signed_url": "%s",
								"method": "%s",
								"form_fields": {%s}
							}]
							}`,
						basePath+"/"+tt.path,
						s.URL+uploadEndpoint,
						tt.wantPresignedURLMethod,
						formFields,
					)

					require.NoError(t, err)
				},
			))

			// This handles the upload of the files with the presigned url that
			// is retrieved from the handler above.
			mux.HandleFunc(uploadEndpoint, http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					defer r.Body.Close() //nolint:errcheck

					assert.Equal(t, tt.wantPresignedURLMethod, r.Method)

					// Does the multipart form data contain the file to upload?
					file, header, err := r.FormFile("file")
					require.NoError(t, err)
					t.Cleanup(func() {
						_ = file.Close()
					})
					cd := header.Header.Get("Content-Disposition")
					assert.Equal(t, cd, `form-data; name="file"; filename="`+basePath+`/`+tt.path+`"`)

					// Does the file content match the expected data?
					bb, err := io.ReadAll(file)
					require.NoError(t, err)
					assert.Equal(t, string(bb), tt.dataToUpload)

					// Is the content type set correctly to the binary data?
					assert.Equal(t, "application/octet-stream", header.Header.Get("Content-Type"))

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
