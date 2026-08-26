package kafka

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildSASL(t *testing.T) {
	t.Parallel()

	// No SASL.
	for _, sc := range []*SASLConfig{nil, {Algorithm: ""}, {Algorithm: saslNone}} {
		m, err := buildSASL(sc, false)
		require.NoError(t, err)
		require.Nil(t, m)
	}

	// PLAIN.
	m, err := buildSASL(&SASLConfig{Algorithm: saslPlain, Username: "u", Password: "p"}, false)
	require.NoError(t, err)
	require.Equal(t, "PLAIN", m.Name())

	// SCRAM.
	m, err = buildSASL(&SASLConfig{Algorithm: saslScramSha256, Username: "u", Password: "p"}, false)
	require.NoError(t, err)
	require.Equal(t, "SCRAM-SHA-256", m.Name())

	m, err = buildSASL(&SASLConfig{Algorithm: saslScramSha512, Username: "u", Password: "p"}, false)
	require.NoError(t, err)
	require.Equal(t, "SCRAM-SHA-512", m.Name())

	// sasl_ssl requires TLS, then uses PLAIN.
	_, err = buildSASL(&SASLConfig{Algorithm: saslSsl, Username: "u", Password: "p"}, false)
	require.Error(t, err)

	m, err = buildSASL(&SASLConfig{Algorithm: saslSsl, Username: "u", Password: "p"}, true)
	require.NoError(t, err)
	require.Equal(t, "PLAIN", m.Name())

	// AWS IAM is deferred to a dedicated change; it errors for now.
	_, err = buildSASL(&SASLConfig{Algorithm: saslAwsIam, AWSProfile: "default"}, false)
	require.ErrorIs(t, err, errAWSIAMNotImplemented)

	// Unknown.
	_, err = buildSASL(&SASLConfig{Algorithm: "bogus"}, false)
	require.Error(t, err)
}
