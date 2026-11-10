package storage

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalFilePersister will persist files to the local disk.
type LocalFilePersister struct{}

// Persist will write the contents of data to the local disk on the specified path.
// TODO: we should not write to disk here but put it on some queue for async disk writes.
func (l *LocalFilePersister) Persist(_ context.Context, path string, data io.Reader) (err error) {
	cp := filepath.Clean(path)

	dir := filepath.Dir(cp)
	if err = os.MkdirAll(dir, 0o755); err != nil { //nolint:forbidigo,gosec
		return fmt.Errorf("creating a local directory %q: %w", dir, err)
	}

	f, err := os.OpenFile(cp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:forbidigo
	if err != nil {
		return fmt.Errorf("creating a local file %q: %w", cp, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("closing the local file %q: %w", cp, cerr)
		}
	}()

	bf := bufio.NewWriter(f)

	if _, err := io.Copy(bf, data); err != nil {
		return fmt.Errorf("copying data to file: %w", err)
	}

	if err := bf.Flush(); err != nil {
		return fmt.Errorf("flushing data to disk: %w", err)
	}

	return nil
}

// RemoteFilePersister is to be used when files created by the browser module need
// to be uploaded to a remote location. This uses a presignedURLRequestURL to
// retrieve one presigned URL. The presigned url is used to upload the file
// to the remote location.
type RemoteFilePersister struct {
	presignedURLRequestURL string
	headers                map[string]string
	basePath               string

	httpClient *http.Client
}

// PresignedURLResponse holds the response from a presigned generation request.
type PresignedURLResponse struct {
	Service string `json:"service"`
	URLs    []struct {
		Name         string            `json:"name"`
		PresignedURL string            `json:"pre_signed_url"` //nolint:tagliatelle
		Method       string            `json:"method"`
		FormFields   map[string]string `json:"form_fields"` //nolint:tagliatelle
	} `json:"urls"`
}

// NewRemoteFilePersister creates a new instance of RemoteFilePersister.
func NewRemoteFilePersister(
	presignedURLRequestURL string,
	headers map[string]string,
	basePath string,
) *RemoteFilePersister {
	return &RemoteFilePersister{
		presignedURLRequestURL: presignedURLRequestURL,
		headers:                headers,
		basePath:               basePath,
		httpClient: &http.Client{
			Timeout: time.Second * 10,
		},
	}
}

// Persist will upload the contents of data to a remote location.
func (r *RemoteFilePersister) Persist(ctx context.Context, path string, data io.Reader) (err error) {
	psResp, err := r.requestPresignedURL(ctx, path)
	if err != nil {
		return fmt.Errorf("requesting presigned url: %w", err)
	}

	req, err := newFileUploadRequest(ctx, psResp, data)
	if err != nil {
		return fmt.Errorf("creating upload request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("performing upload request: %w", err)
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

// requestPresignedURL will request a new presigned URL from the remote server
// and returns a [PresignedURLResponse] that contains the presigned URL details.
func (r *RemoteFilePersister) requestPresignedURL(ctx context.Context, path string) (PresignedURLResponse, error) {
	b, err := buildPresignedRequestBody(r.basePath, path)
	if err != nil {
		return PresignedURLResponse{}, fmt.Errorf("building request body: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		r.presignedURLRequestURL,
		bytes.NewReader(b),
	)
	if err != nil {
		return PresignedURLResponse{}, fmt.Errorf("creating request: %w", err)
	}

	for k, v := range r.headers {
		req.Header.Add(k, v)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return PresignedURLResponse{}, fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if err := checkStatusCode(resp); err != nil {
		return PresignedURLResponse{}, err
	}

	return readPresignedURLResponse(resp)
}

func buildPresignedRequestBody(basePath, path string) ([]byte, error) {
	b := struct {
		Service   string `json:"service"`
		Operation string `json:"operation"`
		Files     []struct {
			Name string `json:"name"`
		} `json:"files"`
	}{
		Service:   "aws_s3",
		Operation: "upload_post",
		Files: []struct {
			Name string `json:"name"`
		}{
			{
				Name: filepath.Join(basePath, path),
			},
		},
	}

	bb, err := json.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("marshaling request body: %w", err)
	}

	return bb, nil
}

func readPresignedURLResponse(resp *http.Response) (PresignedURLResponse, error) {
	var rb PresignedURLResponse

	decoder := json.NewDecoder(resp.Body)
	err := decoder.Decode(&rb)
	if err != nil {
		return PresignedURLResponse{}, fmt.Errorf("decoding response body: %w", err)
	}

	if len(rb.URLs) == 0 {
		return PresignedURLResponse{}, errors.New("missing presigned url in response body")
	}

	return rb, nil
}

// newFileUploadRequest creates a new HTTP request to upload a file as a multipart
// form to the presigned URL received from the server.
func newFileUploadRequest(
	ctx context.Context,
	resp PresignedURLResponse,
	data io.Reader,
) (*http.Request, error) {
	// we don't support multiple presigned URLs at the moment.
	psu := resp.URLs[0]

	// copy all form fields received from a presigned URL
	// response to the multipart form fields.
	var form bytes.Buffer
	fw := multipart.NewWriter(&form)
	for k, v := range psu.FormFields {
		if err := fw.WriteField(k, v); err != nil {
			return nil, fmt.Errorf("writing form field key %q and value %q: %w", k, v, err)
		}
	}
	// attach the file data to the form.
	ff, err := fw.CreateFormFile("file", psu.Name)
	if err != nil {
		return nil, fmt.Errorf("creating multipart form file: %w", err)
	}
	if _, err := io.Copy(ff, data); err != nil {
		return nil, fmt.Errorf("copying file data to multipart form: %w", err)
	}
	if err := fw.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart form writer: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		psu.Method,
		psu.PresignedURL,
		&form,
	)
	if err != nil {
		return nil, fmt.Errorf("creating new request: %w", err)
	}
	req.Header.Set("Content-Type", fw.FormDataContentType())

	return req, nil
}

func checkStatusCode(resp *http.Response) error {
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("server returned %d (%s)", resp.StatusCode, strings.ToLower(http.StatusText(resp.StatusCode)))
	}

	return nil
}
