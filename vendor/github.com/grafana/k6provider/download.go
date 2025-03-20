package k6provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"time"
)

const (
	// DefaultRetries number of retries for download requests
	DefaultRetries = 3
	// DefaultBackoff initial backoff time between retries. It is incremented exponentially between retries.
	DefaultBackoff = 1 * time.Second
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
}

// newDownloader returns a new Downloader
func newDownloader(config DownloadConfig) (*downloader, error) {
	httpClient := http.DefaultClient

	proxyURL := config.ProxyURL
	if proxyURL == "" {
		proxyURL = os.Getenv("K6_DOWNLOAD_PROXY")
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
		downloadAuth = os.Getenv("K6_DOWNLOAD_AUTH")
	}

	downloadAuthType := config.AuthType
	if downloadAuthType == "" {
		downloadAuthType = "Bearer"
	}

	return &downloader{
		client:   httpClient,
		auth:     downloadAuth,
		authType: downloadAuthType,
		headers:  config.Headers,
		retries:  config.Retries,
		backoff:  config.Backoff,
	}, nil
}

//nolint:funlen
func (d *downloader) download(ctx context.Context, from string, path string, checksum string) error {
	downloadBin := path + ".download"
	dest, err := os.OpenFile( //nolint:gosec
		downloadBin,
		os.O_TRUNC|os.O_WRONLY|os.O_CREATE,
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

	var (
		resp    *http.Response
		backoff = d.backoff
		retries = d.retries
	)

	if retries == 0 {
		retries = DefaultRetries
	}

	if backoff == 0 {
		backoff = DefaultBackoff
	}

	// try at least once
	for {
		// it is safe to reuse the request as it doesn't have a body
		resp, err = d.client.Do(req)

		if retries == 0 || !shouldRetry(err, resp) {
			break
		}

		time.Sleep(backoff)

		// increase backoff exponentially for next retry
		backoff *= 2
		retries--
	}

	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %s", resp.Status)
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

	err = os.Rename(downloadBin, path)

	return err
}

// shouldRetry returns true if the error or response indicates that the request should be retried
func shouldRetry(err error, resp *http.Response) bool {
	if err != nil {
		if errors.Is(err, io.EOF) { // assuming EOF is due to connection interrupted by network error
			return true
		}

		var ne net.Error
		if errors.As(err, &ne) {
			return ne.Timeout()
		}

		return false
	}

	if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusInternalServerError {
		return true
	}

	return false
}
