package storage

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating a local directory %q: %w", dir, err)
	}

	f, err := os.OpenFile(cp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
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
// to be uploaded to a remote location. This uses a preSignedURLGetterURL to
// retrieve one pre-signed URL. The pre-signed url is used to upload the file
// to the remote location.
type RemoteFilePersister struct {
	preSignedURLGetterURL string
	headers               map[string]string
	basePath              string

	httpClient *http.Client
}

// NewRemoteFilePersister creates a new instance of RemoteFilePersister.
func NewRemoteFilePersister(
	preSignedURLGetterURL string,
	headers map[string]string,
	basePath string,
) *RemoteFilePersister {
	return &RemoteFilePersister{
		preSignedURLGetterURL: preSignedURLGetterURL,
		headers:               headers,
		basePath:              basePath,
		httpClient: &http.Client{
			Timeout: time.Second * 10,
		},
	}
}

// Persist will upload the contents of data to a remote location.
func (r *RemoteFilePersister) Persist(ctx context.Context, path string, data io.Reader) (err error) {
	pURL, err := r.getPreSignedURL(ctx, path)
	if err != nil {
		return fmt.Errorf("getting presigned url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, pURL, data)
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

func checkStatusCode(resp *http.Response) error {
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("server returned %d (%s)", resp.StatusCode, strings.ToLower(http.StatusText(resp.StatusCode)))
	}

	return nil
}

// getPreSignedURL will retrieve the presigned url for the current file.
func (r *RemoteFilePersister) getPreSignedURL(ctx context.Context, path string) (string, error) {
	b, err := buildPresignedRequestBody(r.basePath, path)
	if err != nil {
		return "", fmt.Errorf("building request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.preSignedURLGetterURL, bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	for k, v := range r.headers {
		req.Header.Add(k, v)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if err := checkStatusCode(resp); err != nil {
		return "", err
	}

	return readResponseBody(resp)
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
		Operation: "upload",
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

func readResponseBody(resp *http.Response) (string, error) {
	rb := struct {
		Service string `json:"service"`
		URLs    []struct {
			Name         string `json:"name"`
			PreSignedURL string `json:"pre_signed_url"` //nolint:tagliatelle
		} `json:"urls"`
	}{}

	decoder := json.NewDecoder(resp.Body)
	err := decoder.Decode(&rb)
	if err != nil {
		return "", fmt.Errorf("decoding response body: %w", err)
	}

	if len(rb.URLs) == 0 {
		return "", errors.New("missing presigned url in response body")
	}

	return rb.URLs[0].PreSignedURL, nil
}
