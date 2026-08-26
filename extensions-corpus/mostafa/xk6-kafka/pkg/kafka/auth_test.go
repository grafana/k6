package kafka

import (
	"crypto/tls"
	"os"
	"testing"

	azcoreFake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/netext"
)

func TestSASLContext(t *testing.T) {
	t.Run("algorithm without context", func(t *testing.T) {
		context, err := NewSaslContext(SASLConfig{
			Algorithm: saslSsl,
		}, []string{"broker1"}, SASLContextOpts{})

		require.NoError(t, err)
		require.Nil(t, context.OAuthProvider)
	})

	t.Run("azure entra algorithm with oauth context", func(t *testing.T) {
		fakeToken := azcoreFake.TokenCredential{}

		opts := SASLContextOpts{
			OAuthProviderOpts: OAuthProviderOpts{
				azureTokenCredential: &fakeToken,
			},
		}

		context, err := NewSaslContext(SASLConfig{
			Algorithm: saslAzureEntra,
		}, []string{"broker1:9093"}, opts)

		require.NoError(t, err)
		require.NotNil(t, context.OAuthProvider)
	})

	t.Run("gcp oauth algorithm with oauth context", func(t *testing.T) {
		opts := SASLContextOpts{
			OAuthProviderOpts: OAuthProviderOpts{
				gcpTokenProvider:   &testGcpTokenProvider{},
				gcpSubjectProvider: &testGcpSubjectProvider{},
			},
		}

		context, err := NewSaslContext(SASLConfig{
			Algorithm: saslGcpOauth,
		}, []string{"broker1:9093"}, opts)

		require.NoError(t, err)
		require.NotNil(t, context.OAuthProvider)
	})
}

func TestConfluentSecurityProtocol(t *testing.T) {
	t.Run("plaintext without tls", func(t *testing.T) {
		protocol, err := confluentSecurityProtocol(SASLConfig{}, TLSConfig{})
		require.NoError(t, err)
		assert.Equal(t, "PLAINTEXT", protocol)
	})

	t.Run("ssl without sasl", func(t *testing.T) {
		protocol, err := confluentSecurityProtocol(SASLConfig{}, TLSConfig{EnableTLS: true})
		require.NoError(t, err)
		assert.Equal(t, "SSL", protocol)
	})

	t.Run("sasl plain with tls", func(t *testing.T) {
		protocol, err := confluentSecurityProtocol(
			SASLConfig{Algorithm: saslPlain},
			TLSConfig{EnableTLS: true},
		)
		require.NoError(t, err)
		assert.Equal(t, "SASL_SSL", protocol)
	})

	t.Run("sasl ssl requires tls", func(t *testing.T) {
		_, err := confluentSecurityProtocol(
			SASLConfig{Algorithm: saslSsl},
			TLSConfig{EnableTLS: false},
		)
		require.Error(t, err)
		assert.EqualError(t, err, "You must enable TLS to use SASL_SSL")
	})

	t.Run("sasl ssl gssapi", func(t *testing.T) {
		protocol, err := confluentSecurityProtocol(
			SASLConfig{Algorithm: saslGssApi},
			TLSConfig{EnableTLS: true},
		)
		require.NoError(t, err)
		assert.Equal(t, "SASL_SSL", protocol)
	})
}

func TestConfluentSASLMechanism(t *testing.T) {
	testCases := map[string]struct {
		algorithm string
		expected  string
	}{
		"plain": {
			algorithm: saslPlain,
			expected:  "PLAIN",
		},
		"sasl gssapi": {
			algorithm: saslGssApi,
			expected:  "GSSAPI",
		},
		"sasl ssl": {
			algorithm: saslSsl,
			expected:  "PLAIN",
		},
		"scram sha 256": {
			algorithm: saslScramSha256,
			expected:  "SCRAM-SHA-256",
		},
		"scram sha 512": {
			algorithm: saslScramSha512,
			expected:  "SCRAM-SHA-512",
		},
		"aws iam": {
			algorithm: saslAwsIam,
			expected:  "AWS_MSK_IAM",
		},
		"azure entra": {
			algorithm: saslAzureEntra,
			expected:  "OAUTHBEARER",
		},
		"gcp oauth": {
			algorithm: saslGcpOauth,
			expected:  "OAUTHBEARER",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			mechanism, err := confluentSASLMechanism(SASLConfig{Algorithm: testCase.algorithm})
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, mechanism)
		})
	}

	t.Run("unsupported mechanism", func(t *testing.T) {
		_, err := confluentSASLMechanism(SASLConfig{Algorithm: "unsupported"})
		require.Error(t, err)
		assert.EqualError(t, err, "Unsupported SASL mechanism for Confluent scaffold")
	})
}

func TestNewTLSObjectMinVersionFromTLSVersions(t *testing.T) {
	t.Parallel()
	require.NotNil(t, TLSVersions)
	cfg := newTLSObject(TLSConfig{
		EnableTLS:             true,
		MinVersion:            netext.TLS_1_3,
		InsecureSkipTLSVerify: true,
	})
	assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
}

// TestFileExists tests the file exists function.
func TestFileExists(t *testing.T) {
	assert.Nil(t, fileExists("auth_test.go"))
	assert.NotNil(t, fileExists("test.go.not"))
}

type SimpleTLSConfig struct {
	tlsConfig TLSConfig
	err       *Xk6KafkaError
}

// TestTlsConfig tests the creation of a TLS config.
func TestTlsConfig(t *testing.T) {
	tlsConfig := TLSConfig{
		EnableTLS:     true,
		ClientCertPem: "testdata/fixtures/client.cer",
		ClientKeyPem:  "testdata/fixtures/client.pem",
		ServerCaPem:   "testdata/fixtures/caroot.cer",
	}
	tlsObject, err := GetTLSConfig(tlsConfig)
	assert.Nil(t, err)
	assert.NotNil(t, tlsObject)
}

// TestTlsConfigFails tests the creation of a TLS config and fails on invalid files and configs.
// nolint: funlen
func TestTlsConfigFails(t *testing.T) {
	tlsConfigs := []*SimpleTLSConfig{
		{
			tlsConfig: TLSConfig{},
			err: &Xk6KafkaError{
				Code:          noTLSConfig,
				Message:       "No TLS config provided. Continuing with TLS disabled.",
				OriginalError: nil,
			},
		},
		{
			tlsConfig: TLSConfig{
				EnableTLS:     true,
				ServerCaPem:   "server.cer",
				ClientCertPem: "client.cer",
				ClientKeyPem:  "client.pem",
			},
			err: &Xk6KafkaError{
				Code:          fileNotFound,
				Message:       "File not found: client.cer",
				OriginalError: nil,
			},
		},
		{
			tlsConfig: TLSConfig{
				EnableTLS:     true,
				ServerCaPem:   "server.cer",
				ClientCertPem: "testdata/fixtures/client.cer",
				ClientKeyPem:  "test.pem",
			},
			err: &Xk6KafkaError{
				Code:          fileNotFound,
				Message:       "File not found: test.pem",
				OriginalError: nil,
			},
		},
		{
			tlsConfig: TLSConfig{
				EnableTLS:     true,
				ServerCaPem:   "server.cer",
				ClientCertPem: "testdata/fixtures/client.cer",
				ClientKeyPem:  "testdata/fixtures/client.pem",
			},
			err: &Xk6KafkaError{
				Code:          fileNotFound,
				Message:       "File not found: server.cer",
				OriginalError: nil,
			},
		},
		{
			tlsConfig: TLSConfig{
				EnableTLS:     true,
				ServerCaPem:   "testdata/fixtures/caroot.cer",
				ClientCertPem: "testdata/fixtures/invalid-client.cer",
				ClientKeyPem:  "testdata/fixtures/invalid-client.pem",
			},
			err: &Xk6KafkaError{
				Code: failedLoadX509KeyPair,
				Message: "Error creating x509 key pair from \"testdata/fixtures/invalid-client.cer\" " +
					"and \"testdata/fixtures/invalid-client.pem\".",
				OriginalError: ErrInvalidPEMData,
			},
		},
		{
			tlsConfig: TLSConfig{
				EnableTLS:     true,
				ClientCertPem: "testdata/fixtures/client.cer",
				ClientKeyPem:  "testdata/fixtures/client.pem",
				ServerCaPem:   "testdata/fixtures/invalid-caroot.cer",
			},
			err: &Xk6KafkaError{
				Code:    failedAppendCaCertFile,
				Message: "Error appending CA certificate file \"testdata/fixtures/invalid-caroot.cer\".",
			},
		},
	}

	for _, c := range tlsConfigs {
		tlsObject, err := GetTLSConfig(c.tlsConfig)
		assert.NotNil(t, err)
		assert.Equal(t, c.err.Code, err.Code)
		assert.Equal(t, c.err.Message, err.Message)
		assert.Nil(t, tlsObject)
	}
}

// TestTlsConfigFromContent tests the creation of a TLS config.
func TestTlsConfigFromContent(t *testing.T) {
	paths := []string{
		"testdata/fixtures/client.cer",
		"testdata/fixtures/client.pem",
		"testdata/fixtures/caroot.cer",
	}
	files := make([]string, len(paths))

	for i, path := range paths {
		file, _ := os.ReadFile(path) // #nosec G304
		files[i] = string(file)
	}

	tlsConfig := TLSConfig{
		EnableTLS:     true,
		ClientCertPem: files[0],
		ClientKeyPem:  files[1],
		ServerCaPem:   files[2],
	}
	tlsObject, err := GetTLSConfig(tlsConfig)

	assert.Nil(t, err)
	assert.NotNil(t, tlsObject)
}
