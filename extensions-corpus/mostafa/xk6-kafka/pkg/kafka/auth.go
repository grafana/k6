package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// TLSVersions is a map of TLS versions to their numeric values.
var TLSVersions map[string]uint16

const (
	none            = "none"
	saslPlain       = "sasl_plain"
	saslGssApi      = "sasl_gssapi"
	saslScramSha256 = "sasl_scram_sha256"
	saslScramSha512 = "sasl_scram_sha512"
	saslSsl         = "sasl_ssl"
	saslAwsIam      = "sasl_aws_iam"
	saslAzureEntra  = "sasl_azure_entra"
	saslGcpOauth    = "sasl_gcp_oauth"
)

type SASLConfig struct {
	Username       string         `json:"username"`
	Password       string         `json:"password"`
	Algorithm      string         `json:"algorithm"`
	AWSProfile     string         `json:"awsProfile"`
	KerberosConfig KerberosConfig `json:"kerberosConfig"`
}

type KerberosConfig struct {
	ServiceName          *string `json:"serviceName,omitempty"`
	Principal            *string `json:"principal,omitempty"`
	KInitCmd             *string `json:"kInitCmd,omitempty"`
	KeyTab               *string `json:"keyTab,omitempty"`
	MinTimeBeforeRelogin *int    `json:"minTimeBeforeRelogin,omitempty"`
}

type SASLContext struct {
	OAuthProvider *OAuthTokenProvider
}

type SASLContextOpts struct {
	OAuthProviderOpts OAuthProviderOpts
}

func NewSaslContext(saslConfig SASLConfig, brokers []string, opts SASLContextOpts) (SASLContext, error) {
	saslContext := SASLContext{}

	switch saslConfig.Algorithm {
	case saslAzureEntra, saslGcpOauth:
		oauthProvider, err := NewOAuthProvider(saslConfig.Algorithm, brokers, opts.OAuthProviderOpts)
		if err != nil {
			return SASLContext{}, err
		}

		saslContext.OAuthProvider = &oauthProvider
	default:
		// nothing to add to the SASL context
	}

	return saslContext, nil
}

type TLSConfig struct {
	EnableTLS             bool   `json:"enableTls"`
	InsecureSkipTLSVerify bool   `json:"insecureSkipTlsVerify"`
	MinVersion            string `json:"minVersion"`
	ClientCertPem         string `json:"clientCertPem"`
	ClientKeyPem          string `json:"clientKeyPem"`
	ServerCaPem           string `json:"serverCaPem"`
}

// GetTLSConfig creates a TLS config from the given TLS config struct and checks for errors.
// nolint: funlen
func GetTLSConfig(tlsConfig TLSConfig) (*tls.Config, *Xk6KafkaError) {
	tlsObject := newTLSObject(tlsConfig)

	if tlsConfig.EnableTLS {
		if tlsConfig.ServerCaPem == "" {
			return tlsObject, nil
		}
	} else {
		// TLS is disabled, and we continue with a unauthenticated dialer
		return nil, NewXk6KafkaError(
			noTLSConfig, "No TLS config provided. Continuing with TLS disabled.", nil)
	}

	if tlsConfig.ClientCertPem != "" && tlsConfig.ClientKeyPem != "" {
		// Try to load the certificates from string
		cert, err := tls.X509KeyPair([]byte(tlsConfig.ClientCertPem), []byte(tlsConfig.ClientKeyPem))
		if err != nil && err.Error() == "tls: failed to find any PEM data in certificate input" {
			// Fall back to loading the client certificate and key from the file
			fileErr := fileExists(tlsConfig.ClientCertPem)
			if fileErr != nil {
				return nil, fileErr
			}

			fileErr = fileExists(tlsConfig.ClientKeyPem)
			if fileErr != nil {
				return nil, fileErr
			}

			cert, err = tls.LoadX509KeyPair(tlsConfig.ClientCertPem, tlsConfig.ClientKeyPem)
			if err != nil {
				return nil, NewXk6KafkaError(
					failedLoadX509KeyPair,
					fmt.Sprintf(
						"Error creating x509 key pair from \"%s\" and \"%s\".",
						tlsConfig.ClientCertPem,
						tlsConfig.ClientKeyPem),
					err)
			}
		} else if err != nil {
			return nil, NewXk6KafkaError(
				failedLoadX509KeyPair,
				"Error creating x509 key pair from passed content.", err)
		}

		tlsObject.Certificates = []tls.Certificate{cert}
	}

	caCertPool := x509.NewCertPool()

	// Load the CA certificate as string if provided
	if ok := caCertPool.AppendCertsFromPEM([]byte(tlsConfig.ServerCaPem)); !ok {
		// Fall back if file path is provided
		err := fileExists(tlsConfig.ServerCaPem)
		if err != nil {
			return nil, err
		}

		caCert, readErr := os.ReadFile(tlsConfig.ServerCaPem)
		if readErr != nil {
			// This might happen on permissions issues or if the file is unreadable somehow
			return nil, NewXk6KafkaError(
				failedReadCaCertFile,
				"Failed to read CA cert file: "+tlsConfig.ServerCaPem,
				readErr)
		}

		if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
			return nil, NewXk6KafkaError(
				failedAppendCaCertFile,
				fmt.Sprintf(
					"Error appending CA certificate file \"%s\".",
					tlsConfig.ServerCaPem),
				nil)
		}
	}

	tlsObject.RootCAs = caCertPool
	return tlsObject, nil
}

// newTLSConfig returns a tls.Config object from the given TLS config.
func newTLSObject(tlsConfig TLSConfig) *tls.Config {
	// Create a TLS config with default settings
	// #nosec G402
	tlsObject := &tls.Config{
		InsecureSkipVerify: tlsConfig.InsecureSkipTLSVerify,
		MinVersion:         tls.VersionTLS12,
	}

	// Set the minimum TLS version
	if tlsConfig.MinVersion != "" {
		if minVersion, ok := TLSVersions[tlsConfig.MinVersion]; ok {
			tlsObject.MinVersion = minVersion
		}
	}

	return tlsObject
}

// fileExists returns nil if the given file exists and error otherwise.
func fileExists(filename string) *Xk6KafkaError {
	_, err := os.Stat(filename)
	if err != nil {
		return NewXk6KafkaError(fileNotFound, "File not found: "+filename, err)
	}
	return nil
}
