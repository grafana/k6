package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"
)

// DirectFilePersister uploads files together with their metadata in a single
// multipart request, without requesting a presigned URL first.
type DirectFilePersister struct {
	uploadURL  string
	headers    map[string]string
	formFields map[string]string
	basePath   string

	httpClient *http.Client
}

// NewDirectFilePersister creates a new instance of DirectFilePersister.
func NewDirectFilePersister(
	uploadURL string,
	headers map[string]string,
	formFields map[string]string,
	basePath string,
) *DirectFilePersister {
	return &DirectFilePersister{
		uploadURL:  uploadURL,
		headers:    headers,
		formFields: formFields,
		basePath:   basePath,
		httpClient: &http.Client{
			Timeout: time.Second * 10,
		},
	}
}

// Persist uploads data as the multipart "file" field, named after basePath and
// path. The configured form fields and the capture time ("captured_at", RFC 3339)
// are sent alongside it.
func (d *DirectFilePersister) Persist(ctx context.Context, path string, data io.Reader) error {
	capturedAt := time.Now().UTC()

	var form bytes.Buffer
	fw := multipart.NewWriter(&form)
	for k, v := range d.formFields {
		if err := fw.WriteField(k, v); err != nil {
			return fmt.Errorf("writing form field key %q: %w", k, err)
		}
	}
	if err := fw.WriteField("captured_at", capturedAt.Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("writing form field key %q: %w", "captured_at", err)
	}
	ff, err := fw.CreateFormFile("file", filepath.ToSlash(filepath.Join(d.basePath, path)))
	if err != nil {
		return fmt.Errorf("creating multipart form file: %w", err)
	}
	if _, err := io.Copy(ff, data); err != nil {
		return fmt.Errorf("copying file data to multipart form: %w", err)
	}
	if err := fw.Close(); err != nil {
		return fmt.Errorf("closing multipart form writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.uploadURL, &form)
	if err != nil {
		return fmt.Errorf("creating upload request: %w", newRequestError(err))
	}
	for k, v := range d.headers {
		req.Header.Add(k, v)
	}
	req.Header.Set("Content-Type", fw.FormDataContentType())

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("performing upload request: %w", newRequestError(err))
	}
	defer resp.Body.Close() //nolint:errcheck

	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("draining upload response body: %w", err)
	}

	if err := checkStatusCode(resp); err != nil {
		return fmt.Errorf("uploading: %w", err)
	}

	return nil
}
