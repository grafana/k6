package k6provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"time"
)

// DownloadConfig defines the configuration for downloading files
type DownloadConfig struct {
	// AuthType type of passed in the header "Authorization: <type> <auth>".
	// Can be used to set the type as "Basic", "Token" or any custom type. Default to "Bearer"
	AuthType string
	// Authorization contain authorization credentials for download requests
	// Passed in the "Authorization <type> <credentials" (see AuthType for the meaning of <type>)
	// If not specified the value of K6_DOWNLOAD_AUTH is used.
	// If no value is defined, the Authentication header is not passed (except is passed as a custom header
	// see Headers)
	Authorization string
	// DownloadHeaders HTTP headers for the download requests
	Headers map[string]string
	// ProxyURL URL to proxy for downloading binaries
	ProxyURL string
	// Retries number of retries for download requests. Default to 3
	Retries int
	// Backoff initial backoff time between retries. Default to 1s
	// It is incremented exponentially between retries: 1s, 2s, 4s...
	Backoff time.Duration
}

// downloader is a utility for downloading files
type downloader struct {
	client   *http.Client
	auth     string
	authType string
	headers  map[string]string
	retries  int
	backoff  time.Duration
	logger   *slog.Logger
}

// newDownloader returns a new Downloader
func newDownloader(config DownloadConfig, logger *slog.Logger) (*downloader, error) {
	httpClient := http.DefaultClient

	proxyURL := config.ProxyURL
	if proxyURL == "" {
		proxyURL = os.Getenv("K6_DOWNLOAD_PROXY") //nolint:forbidigo
	}
	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			return nil, NewWrappedError(ErrConfig, err)
		}
		proxy := http.ProxyURL(parsed)
		transport := &http.Transport{Proxy: proxy}
		httpClient = &http.Client{Transport: transport}
	}

	downloadAuth := config.Authorization
	if downloadAuth == "" {
		downloadAuth = os.Getenv("K6_DOWNLOAD_AUTH") //nolint:forbidigo
	}

	downloadAuthType := config.AuthType
	if downloadAuthType == "" {
		downloadAuthType = "Bearer"
	}

	retries, backoff, err := resolveRetryConfig(config.Retries, config.Backoff)
	if err != nil {
		return nil, NewWrappedError(ErrConfig, fmt.Errorf("download %w", err))
	}

	return &downloader{
		client:   httpClient,
		auth:     downloadAuth,
		authType: downloadAuthType,
		headers:  config.Headers,
		retries:  retries,
		backoff:  backoff,
		logger:   logger,
	}, nil
}

func (d *downloader) download(ctx context.Context, from string, path string, checksum string) error {
	downloadBin := path + ".download"
	dest, err := os.OpenFile( //nolint:gosec,forbidigo
		downloadBin,
		os.O_TRUNC|os.O_WRONLY|os.O_CREATE, //nolint:forbidigo
		syscall.S_IRUSR|syscall.S_IXUSR|syscall.S_IWUSR,
	)
	if err != nil {
		return err
	}

	// ensure we close in case of error
	defer dest.Close() //nolint:errcheck

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, from, nil)
	if err != nil {
		return err
	}

	// add authorization header "Authorization: <type> <auth>"
	if d.auth != "" {
		req.Header.Add("Authorization", fmt.Sprintf("%s %s", d.authType, d.auth))
	}

	// add custom headers
	for h, v := range d.headers {
		req.Header.Add(h, v)
	}

	resp, err := d.doWithRetry(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	// write content to object file and copy to buffer to calculate checksum
	// TODO: optimize memory by copying content in blocks
	buff := bytes.Buffer{}
	_, err = io.Copy(dest, io.TeeReader(resp.Body, &buff))
	if err != nil {
		return err
	}

	err = dest.Close()
	if err != nil {
		return err
	}

	// calculate and validate checksum
	downloadChecksum := fmt.Sprintf("%x", sha256.Sum256(buff.Bytes()))
	if checksum != downloadChecksum {
		return fmt.Errorf("downloaded content checksum mismatch")
	}

	err = os.Rename(downloadBin, path) //nolint:forbidigo

	return err
}

// doWithRetry runs req, retrying with exponential backoff while shouldRetryDownload
// classifies the failure as transient. It reuses req across attempts, which is safe
// because a download request never has a body.
func (d *downloader) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	return withRetry(ctx, d.retries, d.backoff, d.logger, "Download", shouldRetryDownload,
		func() (*http.Response, error) {
			return d.doRequest(req)
		})
}

func (d *downloader) doRequest(req *http.Request) (*http.Response, error) {
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, &StatusError{StatusCode: resp.StatusCode, Status: resp.Status}
	}

	return resp, nil
}

// shouldRetryDownload returns true if the error indicates the download request should be retried:
// a network error (assumed transient) or a 500/503 response status.
func shouldRetryDownload(err error) bool {
	if statusErr, ok := errors.AsType[*StatusError](err); ok {
		return statusErr.StatusCode == http.StatusServiceUnavailable || statusErr.StatusCode == http.StatusInternalServerError
	}

	if errors.Is(err, io.EOF) { // assuming EOF is due to connection interrupted by network error
		return true
	}

	if ne, ok := errors.AsType[net.Error](err); ok {
		return ne.Timeout()
	}

	return false
}
