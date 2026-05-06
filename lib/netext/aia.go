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

// systemCertPoolOnce guards the one-time load of the system cert pool so that we pay
// the syscall cost at most once per process rather than once per TLS handshake.
var ( //nolint:gochecknoglobals
	systemCertPoolOnce sync.Once
	cachedSystemPool   *x509.CertPool

	// aiaIntermediateCache caches successfully fetched intermediate certificates by AIA URL.
	// Repeated TLS handshakes (e.g. many VUs hitting the same server) pay the HTTP cost at
	// most once per URL per process lifetime.
	aiaIntermediateCache sync.Map // map[string]*x509.Certificate
)

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

	// Preserve any pre-existing callbacks so we can chain them.
	prevVerifyPeer := cfg.VerifyPeerCertificate
	prevVerifyConn := cfg.VerifyConnection

	newCfg := cfg.Clone()

	// Disable Go's built-in chain+hostname verification so we can interpose AIA fetching.
	// Hostname verification is restored below in VerifyConnection.
	newCfg.InsecureSkipVerify = true //nolint:gosec

	// The closure reads newCfg.RootCAs at handshake time rather than capturing the pool
	// at wrap time. This allows callers to modify TLSConfig.RootCAs after VU creation
	// (e.g. in tests, or for per-host CA pinning) and have those changes take effect.
	newCfg.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
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

		// Build an intermediates pool from whatever the server sent.
		intermediates := x509.NewCertPool()
		for _, cert := range certs[1:] {
			intermediates.AddCert(cert)
		}

		// Use the live RootCAs from the config. When nil, fall back to the system pool.
		roots := newCfg.RootCAs
		if roots == nil {
			roots = getSystemCertPool(logger)
		}

		opts := x509.VerifyOptions{
			Roots:         roots,
			Intermediates: intermediates,
			// DNSName is intentionally omitted here: hostname verification is done
			// separately in VerifyConnection so we can access cs.ServerName reliably.
		}

		if _, verifyErr := certs[0].Verify(opts); verifyErr != nil {
			// Only try AIA when the error is "unknown authority"; other errors (expired,
			// constraint violation, etc.) won't be fixed by fetching intermediates.
			var unknownAuthErr x509.UnknownAuthorityError
			if !errors.As(verifyErr, &unknownAuthErr) {
				return verifyErr
			}

			// Record whether any certs advertise AIA URLs so we can emit a useful log
			// message if the chain still fails after fetching.
			hasAIAURLs := false
			for _, c := range certs {
				if len(c.IssuingCertificateURL) > 0 {
					hasAIAURLs = true
					break
				}
			}

			fetched := fetchAIAIntermediates(certs, httpClient, logger)
			for _, cert := range fetched {
				intermediates.AddCert(cert)
			}

			if _, retryErr := certs[0].Verify(opts); retryErr != nil {
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
		}

		if prevVerifyPeer != nil {
			// verifiedChains is nil because we bypassed Go's normal verification path.
			return prevVerifyPeer(rawCerts, nil)
		}
		return nil
	}

	// VerifyConnection is called after VerifyPeerCertificate even when InsecureSkipVerify is
	// true. We use it to restore hostname verification that InsecureSkipVerify suppresses.
	// cs.ServerName is populated by Go's TLS stack from config.ServerName (which the HTTP
	// transport sets to the dialled host), so it is correct for both hostname and IP targets.
	newCfg.VerifyConnection = func(cs tls.ConnectionState) error {
		if cs.ServerName != "" && len(cs.PeerCertificates) > 0 {
			if err := cs.PeerCertificates[0].VerifyHostname(cs.ServerName); err != nil {
				return err
			}
		}
		if prevVerifyConn != nil {
			return prevVerifyConn(cs)
		}
		return nil
	}

	return newCfg
}

// fetchAIAIntermediates follows the AIA IssuingCertificateURL fields on the presented
// certificates (BFS, up to aiaMaxFetchDepth hops) and returns any newly found intermediates.
func fetchAIAIntermediates(certs []*x509.Certificate, httpClient *http.Client, logger logrus.FieldLogger) []*x509.Certificate {
	var fetched []*x509.Certificate
	seen := make(map[string]bool)

	queue := make([]*x509.Certificate, len(certs))
	copy(queue, certs)

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
	ctx, cancel := context.WithTimeout(context.Background(), aiaFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building AIA request: %w", err)
	}

	resp, err := httpClient.Do(req)
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
