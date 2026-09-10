package netext

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"
)

const (
	aiaFetchTimeout  = 10 * time.Second
	aiaMaxCertSize   = 64 * 1024
	aiaMaxFetchDepth = 5
	aiaCacheEntryTTL = 24 * time.Hour
)

var (
	systemCertPoolOnce sync.Once      //nolint:gochecknoglobals
	cachedSystemPool   *x509.CertPool //nolint:gochecknoglobals
)

type aiaCacheEntry struct {
	cert     *x509.Certificate
	storedAt time.Time
}

// DialContextFunc matches net.Dialer.DialContext / lib.DialContexter.DialContext.
type DialContextFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// AIAFetcher fetches missing intermediate certificates via AIA. One instance per Runner
// keeps the cache and singleflight group isolated across in-process test runs.
type AIAFetcher struct {
	httpClient *http.Client
	cache      sync.Map
	fetchGroup singleflight.Group
}

// NewAIAFetcher binds the fetcher's HTTP client to dial so AIA fetches respect the
// caller's blocking lists, host overrides and resolver. Nil dial uses net's default —
// intended for tests only.
func NewAIAFetcher(dial DialContextFunc) *AIAFetcher {
	return &AIAFetcher{
		httpClient: &http.Client{
			Timeout: aiaFetchTimeout,
			Transport: &http.Transport{
				Proxy:       http.ProxyFromEnvironment,
				DialContext: dial,
			},
		},
	}
}

// Wrap returns a *tls.Config whose verification callbacks fetch missing intermediates
// via AIA when the server presents an incomplete chain. Returned unchanged when
// InsecureSkipVerify is already set.
func (f *AIAFetcher) Wrap(cfg *tls.Config, logger logrus.FieldLogger) *tls.Config {
	if cfg.InsecureSkipVerify {
		return cfg
	}

	newCfg := cfg.Clone()

	// Suppresses Go's built-in verification so we can interpose AIA fetching;
	// hostname verification is restored in buildVerifyConnFn.
	newCfg.InsecureSkipVerify = true

	// Callbacks read newCfg.RootCAs at handshake time (not capture-at-wrap-time) so
	// callers can modify RootCAs after wrapping and have the change take effect.
	newCfg.VerifyPeerCertificate = f.buildVerifyPeerFn(newCfg, logger)
	newCfg.VerifyConnection = buildVerifyConnFn()
	return newCfg
}

// Returns nil on failure so x509.VerifyOptions falls through to Go's default system-pool
// handling, rather than substituting an empty pool that guarantees verification failure.
func getSystemCertPool(logger logrus.FieldLogger) *x509.CertPool {
	systemCertPoolOnce.Do(func() {
		pool, err := x509.SystemCertPool()
		if err != nil {
			logger.WithError(err).Debug("AIA: failed to load system cert pool")
			return
		}
		cachedSystemPool = pool
	})
	return cachedSystemPool
}

func (f *AIAFetcher) buildVerifyPeerFn(
	cfg *tls.Config,
	logger logrus.FieldLogger,
) func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return errors.New("tls: server presented no certificates")
		}

		certs := make([]*x509.Certificate, 0, len(rawCerts))
		for _, raw := range rawCerts {
			cert, err := x509.ParseCertificate(raw)
			if err != nil {
				return fmt.Errorf("tls: failed to parse server certificate: %w", err)
			}
			certs = append(certs, cert)
		}

		intermediates := x509.NewCertPool()
		for _, c := range certs[1:] {
			intermediates.AddCert(c)
		}

		roots := cfg.RootCAs
		if roots == nil {
			roots = getSystemCertPool(logger)
		}

		opts := x509.VerifyOptions{
			Roots:         roots,
			Intermediates: intermediates,
			// Hostname verification runs in buildVerifyConnFn where cs.ServerName is available.
		}

		return f.verifyWithAIA(certs, opts, intermediates, logger)
	}
}

func (f *AIAFetcher) verifyWithAIA(
	certs []*x509.Certificate,
	opts x509.VerifyOptions,
	intermediates *x509.CertPool,
	logger logrus.FieldLogger,
) error {
	_, verifyErr := certs[0].Verify(opts)
	if verifyErr == nil {
		return nil
	}

	// Only unknown-authority errors are AIA-fixable; expired, hostname mismatch, etc. aren't.
	var unknownAuthErr x509.UnknownAuthorityError
	if !errors.As(verifyErr, &unknownAuthErr) {
		return verifyErr
	}

	hasAIAURLs := false
	for _, c := range certs {
		if len(c.IssuingCertificateURL) > 0 {
			hasAIAURLs = true
			break
		}
	}

	for _, cert := range f.fetchAIAIntermediates(certs, logger) {
		intermediates.AddCert(cert)
	}

	_, retryErr := certs[0].Verify(opts)
	if retryErr == nil {
		return nil
	}
	if hasAIAURLs {
		logger.WithError(retryErr).Warn(
			"AIA: certificate chain incomplete after fetching intermediates; " +
				"verify that the AIA endpoint is reachable and returns a valid certificate",
		)
	}
	return retryErr
}

func buildVerifyConnFn() func(tls.ConnectionState) error {
	return func(cs tls.ConnectionState) error {
		// Empty ServerName = IP-literal target (Go strips IPs from SNI). We've disabled
		// stdlib chain-and-hostname verification to interpose AIA, so fail closed rather
		// than accept any valid chain.
		if cs.ServerName == "" {
			return errors.New(
				"tlsAIAFetch: cannot verify hostname for IP-literal target " +
					"(SNI is not sent for IP addresses); use a hostname, or disable tlsAIAFetch")
		}
		return cs.PeerCertificates[0].VerifyHostname(cs.ServerName)
	}
}

// The depth limit bounds HTTP fetches, not queue pops — cached items keep flowing even
// after we stop issuing new fetches.
func (f *AIAFetcher) fetchAIAIntermediates(
	certs []*x509.Certificate, logger logrus.FieldLogger,
) []*x509.Certificate {
	var fetched []*x509.Certificate
	seen := make(map[string]bool)
	fetches := 0

	queue := make([]*x509.Certificate, 0, len(certs))
	queue = append(queue, certs...)

	for len(queue) > 0 {
		cert := queue[0]
		queue = queue[1:]

		for _, rawURL := range cert.IssuingCertificateURL {
			if seen[rawURL] {
				continue
			}
			seen[rawURL] = true

			issuer, ok := f.loadCachedAIACert(rawURL)
			if !ok {
				if fetches >= aiaMaxFetchDepth {
					continue
				}
				fetches++
				var err error
				issuer, err = f.fetchAndCacheAIACert(rawURL, logger)
				if err != nil {
					logger.WithError(err).WithField("url", rawURL).Debug("AIA intermediate certificate fetch failed")
					continue
				}
			}

			fetched = append(fetched, issuer)
			queue = append(queue, issuer)
		}
	}

	return fetched
}

func (f *AIAFetcher) loadCachedAIACert(rawURL string) (*x509.Certificate, bool) {
	raw, ok := f.cache.Load(rawURL)
	if !ok {
		return nil, false
	}
	entry := raw.(*aiaCacheEntry) //nolint:forcetypeassert
	// Evict on TTL expiry or when the cert itself is past NotAfter — an expired
	// intermediate won't validate anyway, and holding it would just waste retries.
	if time.Since(entry.storedAt) > aiaCacheEntryTTL || time.Now().After(entry.cert.NotAfter) {
		f.cache.Delete(rawURL)
		return nil, false
	}
	return entry.cert, true
}

func (f *AIAFetcher) fetchAndCacheAIACert(
	rawURL string, logger logrus.FieldLogger,
) (*x509.Certificate, error) {
	v, err, _ := f.fetchGroup.Do(rawURL, func() (any, error) {
		// Re-check under the singleflight barrier: an earlier waiter may have populated it.
		if cert, ok := f.loadCachedAIACert(rawURL); ok {
			return cert, nil
		}
		cert, err := f.fetchCertFromAIAURL(rawURL, logger)
		if err != nil {
			return nil, err
		}
		f.cache.Store(rawURL, &aiaCacheEntry{cert: cert, storedAt: time.Now()})
		return cert, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*x509.Certificate), nil //nolint:forcetypeassert
}

func (f *AIAFetcher) fetchCertFromAIAURL(
	rawURL string, logger logrus.FieldLogger,
) (*x509.Certificate, error) {
	// VerifyPeerCertificate provides no context to thread through; use our own timeout.
	ctx, cancel := context.WithTimeout(context.Background(), aiaFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building AIA request: %w", err)
	}

	resp, err := f.httpClient.Do(req) //nolint:gosec // G107: URL is from a server-presented cert AIA extension
	if err != nil {
		return nil, fmt.Errorf("fetching AIA certificate: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AIA endpoint returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, aiaMaxCertSize))
	if err != nil {
		return nil, fmt.Errorf("reading AIA response: %w", err)
	}

	if cert, parseErr := x509.ParseCertificate(body); parseErr == nil {
		return cert, nil
	}

	block, _ := pem.Decode(body)
	if block == nil {
		if isLikelyPKCS7(resp.Header.Get("Content-Type"), body) {
			logger.WithField("url", rawURL).WithField("content-type", resp.Header.Get("Content-Type")).
				Warn("AIA response is PKCS#7 (not currently supported); chain will remain incomplete")
		}
		return nil, errors.New("AIA response is neither valid DER nor PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing PEM certificate from AIA response: %w", err)
	}
	return cert, nil
}

func isLikelyPKCS7(contentType string, body []byte) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "pkcs7") || strings.Contains(ct, "pkcs-7") {
		return true
	}
	// ASN.1 encoding of OID 1.2.840.113549.1.7.2 (id-signedData).
	return bytes.Contains(body, []byte{0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x07, 0x02})
}
