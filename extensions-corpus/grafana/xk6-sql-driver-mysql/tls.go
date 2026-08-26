// Adapted more or less unchanged from: https://github.com/grafana/xk6-sql/blob/v0.4.1/sql.go
// It will have to be refactored.

package mysql

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/go-sql-driver/mysql"
	"go.k6.io/k6/v2/lib/netext"
)

// supportedTLSVersions is a map of TLS versions to their numeric values.
var supportedTLSVersions = map[string]uint16{ //nolint: gochecknoglobals
	netext.TLS_1_0: tls.VersionTLS10,
	netext.TLS_1_1: tls.VersionTLS11,
	netext.TLS_1_2: tls.VersionTLS12,
	netext.TLS_1_3: tls.VersionTLS13,
}

// tlsExports add TLS related exports to name exports.
func (mod *module) tlsExports() {
	// TLS versions
	mod.exports.Named["TLS_1_0"] = netext.TLS_1_0
	mod.exports.Named["TLS_1_1"] = netext.TLS_1_1
	mod.exports.Named["TLS_1_2"] = netext.TLS_1_2
	mod.exports.Named["TLS_1_3"] = netext.TLS_1_3

	// functions
	mod.exports.Named["loadTLS"] = mod.LoadTLS
	mod.exports.Named["addTLS"] = mod.AddTLS
}

const tlsConfigKey = "custom"

// TLSConfig contains all the TLS configuration options passed between the JS and Go code.
type TLSConfig struct {
	EnableTLS             bool   `json:"enableTLS"`
	InsecureSkipTLSverify bool   `json:"insecureSkipTLSverify"`
	MinVersion            string `json:"minVersion"`
	CAcertFile            string `json:"caCertFile"`
	ClientCertFile        string `json:"clientCertFile"`
	ClientKeyFile         string `json:"clientKeyFile"`
}

// LoadTLS loads the TLS configuration for the SQL module.
func (mod *module) LoadTLS(params map[string]interface{}) error {
	var tlsConfig *TLSConfig
	if b, err := json.Marshal(params); err != nil {
		return err
	} else {
		if err := json.Unmarshal(b, &tlsConfig); err != nil {
			return err
		}
	}
	if _, ok := supportedTLSVersions[tlsConfig.MinVersion]; !ok {
		return fmt.Errorf("unsupported TLS version: %s", tlsConfig.MinVersion)
	}
	mod.tlsConfig = *tlsConfig

	if tlsConfig.EnableTLS {
		if err := registerTLS(tlsConfigKey, mod.tlsConfig); err != nil {
			return err
		}
	}

	return nil
}

// AddTLS add the "tls" connection parameter if TLS is enabled.
func (mod *module) AddTLS(connectionString string) string {
	if mod.tlsConfig.EnableTLS {
		connectionString = prefixConnectionString(connectionString, tlsConfigKey)
	}

	return connectionString
}

// prefixConnectionString prefixes the connection string with the TLS configuration key.
func prefixConnectionString(connectionString string, tlsConfigKey string) string {
	tlsParam := fmt.Sprintf("tls=%s", tlsConfigKey)
	if strings.Contains(connectionString, tlsParam) {
		return connectionString
	}
	var separator string
	if strings.Contains(connectionString, "?") {
		separator = "&"
	} else {
		separator = "?"
	}
	return fmt.Sprintf("%s%s%s", connectionString, separator, tlsParam)
}

// registerTLS loads the ca-cert and registers the TLS configuration with the MySQL driver.
func registerTLS(tlsConfigKey string, tlsConfig TLSConfig) error {
	rootCAs := x509.NewCertPool()
	pem, err := os.ReadFile(tlsConfig.CAcertFile) //nolint: forbidigo
	if err != nil {
		return err
	}
	if ok := rootCAs.AppendCertsFromPEM(pem); !ok {
		return fmt.Errorf("failed to append PEM")
	}

	clientCerts := make([]tls.Certificate, 0, 1)
	certs, err := tls.LoadX509KeyPair(tlsConfig.ClientCertFile, tlsConfig.ClientKeyFile)
	if err != nil {
		return err
	}
	clientCerts = append(clientCerts, certs)

	mysqlTLSConfig := &tls.Config{
		RootCAs:            rootCAs,
		Certificates:       clientCerts,
		MinVersion:         supportedTLSVersions[tlsConfig.MinVersion],
		InsecureSkipVerify: tlsConfig.InsecureSkipTLSverify, // #nosec G402
	}
	return mysql.RegisterTLSConfig(tlsConfigKey, mysqlTLSConfig)
}
