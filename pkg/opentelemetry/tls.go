package opentelemetry

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	"gopkg.in/guregu/null.v3"
)

func buildTLSConfig(
	insecureSkipVerify null.Bool,
	certPath, clientCertPath, clientKeyPath null.String,
) (*tls.Config, error) {
	set := false
	tlsConfig := &tls.Config{} //nolint:gosec // it's up to the caller

	if insecureSkipVerify.Valid {
		tlsConfig.InsecureSkipVerify = insecureSkipVerify.Bool
		set = true
	}

	// Load the root certificate
	if certPath.Valid {
		b, err := os.ReadFile(certPath.String) //nolint:forbidigo
		if err != nil {
			return nil, fmt.Errorf("failed to read root certificate from %q: %w", certPath.String, err)
		}

		cp := x509.NewCertPool()
		if ok := cp.AppendCertsFromPEM(b); !ok {
			return nil, errors.New("failed to append root certificate to the pool")
		}

		tlsConfig.RootCAs = cp
		set = true
	}

	// Load the client certificate
	if clientCertPath.Valid {
		cert, err := tls.LoadX509KeyPair(clientCertPath.String, clientKeyPath.String)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}

		tlsConfig.Certificates = []tls.Certificate{cert}
		set = true
	}

	if !set {
		return nil, nil //nolint:nilnil // no TLS config set
	}

	return tlsConfig, nil
}
