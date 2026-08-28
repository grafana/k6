package kafka

import (
	"testing"

	"github.com/stretchr/testify/require"
	extensionapitest "go.k6.io/k6-extension-api/test"
)

func TestClientOptions(t *testing.T) {
	t.Parallel()

	vu := extensionapitest.NewVU()
	// Brokers only, no SASL/TLS.
	opts, err := clientOptions(vu, []string{"localhost:9092"}, nil, nil)
	require.NoError(t, err)
	require.NotEmpty(t, opts)

	// SASL + TLS produce additional options.
	keyDER, certDER := genKeyCert(t)
	withAuth, err := clientOptions(
		vu,
		[]string{"localhost:9092"},
		&SASLConfig{Algorithm: saslPlain, Username: "u", Password: "p"},
		&TLSConfig{EnableTLS: true, ClientCertPem: pemStr("CERTIFICATE", certDER), ClientKeyPem: pemStr("PRIVATE KEY", keyDER)},
	)
	require.NoError(t, err)
	require.Greater(t, len(withAuth), len(opts))

	// Invalid TLS bubbles up.
	_, err = clientOptions(vu, []string{"localhost:9092"}, nil, &TLSConfig{EnableTLS: true, MinVersion: "bogus"})
	require.Error(t, err)
}
