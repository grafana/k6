package httpext

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"go.k6.io/k6/v2/lib"
)

// withTLSClientCertificates returns a VU state that presents the given client
// certificates for TLS handshakes. When certs is empty the original state is
// returned unchanged. The returned cleanup closes idle connections on any
// cloned transport so per-request certificate pools do not leak.
func withTLSClientCertificates(state *lib.State, certs []tls.Certificate) (*lib.State, func(), error) {
	noop := func() {}
	if len(certs) == 0 {
		return state, noop, nil
	}

	base, ok := state.Transport.(*http.Transport)
	if !ok {
		return nil, nil, fmt.Errorf(
			"cannot apply per-request tlsAuth: unexpected transport type %T", state.Transport)
	}

	cloned := base.Clone()
	var tlsCfg *tls.Config
	if state.TLSConfig != nil {
		tlsCfg = state.TLSConfig.Clone()
	} else {
		tlsCfg = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	tlsCfg.Certificates = certs
	// NameToCertificate is server-oriented and ineffective for clients; clear it
	// so the explicit Certificates list is what GetClientCertificate uses.
	tlsCfg.NameToCertificate = nil //nolint:staticcheck
	cloned.TLSClientConfig = tlsCfg

	stateCopy := *state
	stateCopy.Transport = cloned
	stateCopy.TLSConfig = tlsCfg
	return &stateCopy, cloned.CloseIdleConnections, nil
}
