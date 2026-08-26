package kafka

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	gcpAuth "cloud.google.com/go/auth"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

var (
	errFakeFailedRetrieveToken = errors.New("failed to retrieve token")
	testSigningKey             = []byte("test-signing-key-unit-test-only")
)

type testAzureEntraTokenCredential struct {
	Subject string
	Error   error
}

var _ azcore.TokenCredential = (*testAzureEntraTokenCredential)(nil)

func (t *testAzureEntraTokenCredential) GetToken(
	ctx context.Context,
	opts policy.TokenRequestOptions,
) (azcore.AccessToken, error) {
	if t.Error != nil {
		return azcore.AccessToken{}, t.Error
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": t.Subject,
	})

	tokenStr, err := token.SignedString(testSigningKey)
	if err != nil {
		return azcore.AccessToken{}, err
	}

	return azcore.AccessToken{Token: tokenStr, ExpiresOn: time.Now().Add(24 * time.Hour)}, nil
}

type testGcpTokenProvider struct {
	Error error
}

var _ gcpAuth.TokenProvider = (*testGcpTokenProvider)(nil)

type testGcpSubjectProvider struct {
	Subject string
	Error   error
}

var _ GcpSubjectProvider = (*testGcpSubjectProvider)(nil)

func (t *testGcpTokenProvider) Token(context.Context) (*gcpAuth.Token, error) {
	if t.Error != nil {
		return nil, t.Error
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{})

	tokenStr, err := token.SignedString(testSigningKey)
	if err != nil {
		return &gcpAuth.Token{}, err
	}

	return &gcpAuth.Token{
		Value:  tokenStr,
		Expiry: time.Now().Add(24 * time.Hour),
	}, nil
}

func (t *testGcpSubjectProvider) GetSubject(ctx context.Context, token string) (string, error) {
	if t.Error != nil {
		return "", t.Error
	}

	return t.Subject, nil
}

func TestGetOAuthTokenSuccess(t *testing.T) {
	testCases := map[string]struct {
		saslAlgorithm        string
		opts                 OAuthProviderOpts
		expectedSubject      string
		expectedExpiredAfter time.Time
		expectedRefreshAfter time.Time
	}{
		"azure entra": {
			saslAlgorithm: saslAzureEntra,
			opts: OAuthProviderOpts{
				azureTokenCredential: &testAzureEntraTokenCredential{
					Subject: "test-sub",
				},
			},
			expectedSubject:      "test-sub",
			expectedExpiredAfter: time.Now(),
			expectedRefreshAfter: time.Time{},
		},
		"gcp oauth": {
			saslAlgorithm: saslGcpOauth,
			opts: OAuthProviderOpts{
				gcpTokenProvider: &testGcpTokenProvider{},
				gcpSubjectProvider: &testGcpSubjectProvider{
					Subject: "test-sub",
				},
			},
			expectedSubject:      "test-sub",
			expectedExpiredAfter: time.Now(),
			expectedRefreshAfter: time.Time{},
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			provider, err := NewOAuthProvider(test.saslAlgorithm, []string{"broker1:9093"}, test.opts)
			require.NoError(t, err)

			token, err := provider.GetToken(t.Context())

			require.NoError(t, err)
			require.NotEmpty(t, token.Token)
			require.Equal(t, test.expectedSubject, token.Subject)
			require.GreaterOrEqual(t, token.ExpiresOn, test.expectedExpiredAfter)
			require.GreaterOrEqual(t, token.RefreshOn, test.expectedRefreshAfter)
		})
	}
}

func TestBuildGcpKafkaToken(t *testing.T) {
	accessToken := "ya29.test-access-token"
	expiresOn := time.Now().Add(1 * time.Hour)
	subject := "test-subject@example.com"

	tokenStr, err := buildGcpKafkaToken(accessToken, expiresOn, subject)
	require.NoError(t, err)
	require.NotEmpty(t, tokenStr)

	parts := strings.Split(tokenStr, ".")
	require.Len(t, parts, 3)

	// Decode Header
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	var header map[string]string
	err = json.Unmarshal(headerBytes, &header)
	require.NoError(t, err)
	require.Equal(t, "JWT", header["typ"])
	require.Equal(t, "GOOG_OAUTH2_TOKEN", header["alg"])

	// Decode Payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var payload map[string]any
	err = json.Unmarshal(payloadBytes, &payload)
	require.NoError(t, err)
	require.Equal(t, "kafka", payload["scope"])
	require.Equal(t, subject, payload["sub"])
	require.Equal(t, float64(expiresOn.Unix()), payload["exp"])
	require.Contains(t, payload, "iat")

	// Decode Token
	rawTokenBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err)
	require.Equal(t, accessToken, string(rawTokenBytes))
}

func TestGetOAuthTokenFailure(t *testing.T) {
	testCases := map[string]struct {
		saslAlgorithm string
		opts          OAuthProviderOpts
	}{
		"azure entra": {
			saslAlgorithm: saslAzureEntra,
			opts: OAuthProviderOpts{
				azureTokenCredential: &testAzureEntraTokenCredential{
					Error: errFakeFailedRetrieveToken,
				},
			},
		},
		"gcp oauth": {
			saslAlgorithm: saslGcpOauth,
			opts: OAuthProviderOpts{
				gcpTokenProvider: &testGcpTokenProvider{
					Error: errFakeFailedRetrieveToken,
				},
				gcpSubjectProvider: &testGcpSubjectProvider{
					Subject: "test-subject",
				},
			},
		},
		"gcp oauth subject": {
			saslAlgorithm: saslGcpOauth,
			opts: OAuthProviderOpts{
				gcpTokenProvider: &testGcpTokenProvider{},
				gcpSubjectProvider: &testGcpSubjectProvider{
					Error: errFakeFailedRetrieveToken,
				},
			},
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			provider, err := NewOAuthProvider(test.saslAlgorithm, []string{"broker1:9093"}, test.opts)
			require.NoError(t, err)

			_, err = provider.GetToken(t.Context())

			var xk6KafkaError *Xk6KafkaError
			ok := errors.As(err, &xk6KafkaError)

			require.True(t, ok, "error is not Xk6KafkaError")
			require.Equal(t, failedGetOAuthToken, xk6KafkaError.Code)
		})
	}
}

func TestUnsupportedOAuthProvider(t *testing.T) {
	_, err := NewOAuthProvider(saslPlain, []string{"broker1"}, OAuthProviderOpts{})
	require.ErrorContains(t, err, "sasl_plain is not a supported OAuth Provider.")
}
