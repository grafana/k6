package netext

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	aiaFetchTimeout  = 10 * time.Second
	aiaMaxCertSize   = 64 * 1024 // 64 KB per intermediate – far more than any real cert needs
	aiaMaxFetchDepth = 5         // how many AIA hops to follow up the issuer chain
)

// aiaHTTPClient is the default HTTP client used for fetching intermediate certificates
// via AIA URLs. It is intentionally separate from k6's test transport so that AIA fetches
// are never measured as test traffic and don't inherit k6's TLS config (circular dependency).
// AIA URLs in the wild are almost always plain HTTP; using HTTPS would create a circular
// dependency (you'd need a certificate to fetch the certificate).
var aiaHTTPClient = &http.Client{ //nolint:gochecknoglobals
	Timeout: aiaFetchTimeout,
}

// systemCertPoolOnce / cachedSystemPool guard the one-time load of the system cert pool
// so that we pay the syscall cost at most once per process rather than once per TLS handshake.
var (
	systemCertPoolOnce sync.Once      //nolint:gochecknoglobals
	cachedSystemPool   *x509.CertPool //nolint:gochecknoglobals
)

// aiaIntermediateCache caches successfully fetched intermediate certificates by AIA URL.
// Repeated TLS handshakes (e.g. many VUs hitting the same server) pay the HTTP cost at
// most once per URL per process lifetime.
var aiaIntermediateCache sync.Map //nolint:gochecknoglobals

func getSystemCertPool(logger logrus.FieldLogger) *x509.CertPool {
	systemCertPoolOnce.Do(func() {
		pool, err := x509.SystemCertPool()
		if err != nil {
			logger.WithError(err).Debug("AIA: failed to load system cert pool, using empty pool")
			pool = x509.NewCertPool()
		}
		cachedSystemPool = pool
	})
	return cachedSystemPool
}

// WrapTLSConfigForAIAFetching returns a new *tls.Config that transparently fetches missing
// intermediate certificates via the AIA (Authority Information Access) extension when the
// server presents an incomplete chain.
//
// Go's crypto/tls does not perform AIA chain fetching; browsers do. Servers that rely on AIA
// to fill in the gap (rather than sending the full chain) will cause Go TLS handshakes to fail
// with "x509: certificate signed by unknown authority". This wrapper re-implements the chain
// and hostname checks so that AIA fetching can be inserted between the first failed attempt
// and the returned error.
//
// httpClient is the HTTP client used to fetch intermediate certificates from AIA URLs.
// Pass nil to use the default client (10-second timeout, no special transport).
//
// When InsecureSkipVerify is already true the config is returned unchanged – the user has
// already opted out of all certificate validation, so there is nothing to fix.
func WrapTLSConfigForAIAFetching(cfg *tls.Config, logger logrus.FieldLogger, httpClient *http.Client) *tls.Config {
	if cfg.InsecureSkipVerify {
		return cfg
	}
	if httpClient == nil {
		httpClient = aiaHTTPClient
	}

	prevVerifyPeer := cfg.VerifyPeerCertificate
	prevVerifyConn := cfg.VerifyConnection

	newCfg := cfg.Clone()

	// Disable Go's built-in chain+hostname verification so we can interpose AIA fetching.
	// Hostname verification is restored in buildVerifyConnFn below.
	newCfg.InsecureSkipVerify = true

	// The callbacks read newCfg.RootCAs at handshake time rather than capturing the pool
	// at wrap time. This allows callers to modify TLSConfig.RootCAs after VU creation
	// (e.g. in tests, or for per-host CA pinning) and have those changes take effect.
	newCfg.VerifyPeerCertificate = buildVerifyPeerFn(newCfg, logger, httpClient, prevVerifyPeer)
	newCfg.VerifyConnection = buildVerifyConnFn(prevVerifyConn)
	return newCfg
}

// buildVerifyPeerFn returns a VerifyPeerCertificate callback that re-implements chain
// verification with AIA fetching inserted between the first failed attempt and the returned
// error. cfg is the cloned config whose RootCAs field is read live on each handshake.
func buildVerifyPeerFn(
	cfg *tls.Config,
	logger logrus.FieldLogger,
	httpClient *http.Client,
	prev func([][]byte, [][]*x509.Certificate) error,
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

		// Use the live RootCAs from the config. When nil, fall back to the system pool.
		roots := cfg.RootCAs
		if roots == nil {
			roots = getSystemCertPool(logger)
		}

		opts := x509.VerifyOptions{
			Roots:         roots,
			Intermediates: intermediates,
			// DNSName is intentionally omitted here: hostname verification is done
			// separately in buildVerifyConnFn so we can access cs.ServerName reliably.
		}

		if err := verifyWithAIA(certs, opts, intermediates, httpClient, logger); err != nil {
			return err
		}

		if prev != nil {
			// verifiedChains is nil because we bypassed Go's normal verification path.
			return prev(rawCerts, nil)
		}
		return nil
	}
}

// verifyWithAIA attempts chain verification and, on x509.UnknownAuthorityError only,
// fetches missing intermediates via AIA and retries.
func verifyWithAIA(
	certs []*x509.Certificate,
	opts x509.VerifyOptions,
	intermediates *x509.CertPool,
	httpClient *http.Client,
	logger logrus.FieldLogger,
) error {
	_, verifyErr := certs[0].Verify(opts)
	if verifyErr == nil {
		return nil
	}

	// Only attempt AIA for unknown-authority errors; expired certs, hostname mismatches,
	// etc. are not fixed by fetching intermediates.
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

	for _, cert := range fetchAIAIntermediates(certs, httpClient, logger) {
		intermediates.AddCert(cert)
	}

	_, retryErr := certs[0].Verify(opts)
	if retryErr == nil {
		return nil
	}
	if hasAIAURLs {
		// Log at Warn so users can diagnose AIA failures without --log-level=debug.
		// Individual per-URL errors are logged at Debug inside fetchAIAIntermediates.
		logger.WithError(retryErr).Warn(
			"AIA: certificate chain incomplete after fetching intermediates; " +
				"verify that the AIA endpoint is reachable and returns a valid certificate",
		)
	}
	return retryErr
}

// buildVerifyConnFn returns a VerifyConnection callback that restores hostname verification
// (suppressed by InsecureSkipVerify) and chains any pre-existing callback.
// cs.ServerName is populated by Go's TLS stack from config.ServerName, which the HTTP
// transport sets to the dialled host — correct for both hostname and IP targets.
func buildVerifyConnFn(prev func(tls.ConnectionState) error) func(tls.ConnectionState) error {
	return func(cs tls.ConnectionState) error {
		if cs.ServerName != "" && len(cs.PeerCertificates) > 0 {
			if err := cs.PeerCertificates[0].VerifyHostname(cs.ServerName); err != nil {
				return err
			}
		}
		if prev != nil {
			return prev(cs)
		}
		return nil
	}
}

// fetchAIAIntermediates follows the AIA IssuingCertificateURL fields on the presented
// certificates (BFS, up to aiaMaxFetchDepth hops) and returns any newly found intermediates.
func fetchAIAIntermediates(
	certs []*x509.Certificate, httpClient *http.Client, logger logrus.FieldLogger,
) []*x509.Certificate {
	var fetched []*x509.Certificate
	seen := make(map[string]bool)

	queue := make([]*x509.Certificate, 0, len(certs))
	queue = append(queue, certs...)

	for depth := 0; depth < aiaMaxFetchDepth && len(queue) > 0; depth++ {
		cert := queue[0]
		queue = queue[1:]

		for _, rawURL := range cert.IssuingCertificateURL {
			if seen[rawURL] {
				continue
			}
			seen[rawURL] = true

			if cached, ok := aiaIntermediateCache.Load(rawURL); ok {
				issuer := cached.(*x509.Certificate) //nolint:forcetypeassert
				fetched = append(fetched, issuer)
				queue = append(queue, issuer)
				continue
			}

			issuer, err := fetchCertFromAIAURL(rawURL, httpClient)
			if err != nil {
				logger.WithError(err).WithField("url", rawURL).Debug("AIA intermediate certificate fetch failed")
				continue
			}

			aiaIntermediateCache.Store(rawURL, issuer)
			fetched = append(fetched, issuer)
			queue = append(queue, issuer)
		}
	}

	return fetched
}

// fetchCertFromAIAURL retrieves a single X.509 certificate (DER or PEM) from rawURL.
func fetchCertFromAIAURL(rawURL string, httpClient *http.Client) (*x509.Certificate, error) {
	// AIA fetches use their own timeout context independent of any VU lifecycle context;
	// the TLS VerifyPeerCertificate callback provides no context to thread through.
	ctx, cancel := context.WithTimeout(context.Background(), aiaFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building AIA request: %w", err)
	}

	resp, err := httpClient.Do(req) //nolint:gosec // G107: URL comes from a server-presented certificate AIA extension
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

	// Servers typically serve intermediates as raw DER; try that first.
	if cert, parseErr := x509.ParseCertificate(body); parseErr == nil {
		return cert, nil
	}

	block, _ := pem.Decode(body)
	if block == nil {
		return nil, errors.New("AIA response is neither valid DER nor PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing PEM certificate from AIA response: %w", err)
	}
	return cert, nil
}
