package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

// CapturedAtField is the multipart form field that carries the capture time.
const CapturedAtField = "captured_at"

// DirectFilePersister uploads files together with their metadata in a single
// multipart request, without requesting a presigned URL first.
type DirectFilePersister struct {
	uploadURL  string
	headers    map[string]string
	formFields map[string]string
	basePath   string
	logger     logrus.FieldLogger

	httpClient *http.Client
}

// NewDirectFilePersister creates a new instance of DirectFilePersister.
func NewDirectFilePersister(
	uploadURL string,
	headers map[string]string,
	formFields map[string]string,
	basePath string,
	logger logrus.FieldLogger,
) *DirectFilePersister {
	return &DirectFilePersister{
		uploadURL:  uploadURL,
		headers:    headers,
		formFields: formFields,
		basePath:   basePath,
		logger:     logger,
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
	if err := fw.WriteField(CapturedAtField, capturedAt.Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("writing form field key %q: %w", CapturedAtField, err)
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

	if err := checkStatusCode(resp); err != nil {
		return fmt.Errorf("uploading: %w", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading upload response body: %w", err)
	}

	// The upload succeeded, so a response body that isn't the expected
	// JSON only means there's no UUID to log.
	var rb struct {
		UUID string `json:"uuid"`
	}
	_ = json.Unmarshal(body, &rb)
	logUploadedUUID(d.logger, rb.UUID)

	return nil
}
