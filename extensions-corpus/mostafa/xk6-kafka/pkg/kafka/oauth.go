package kafka

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"time"

	gcpAuth "cloud.google.com/go/auth"
	gcpCredentials "cloud.google.com/go/auth/credentials"

	googleOAuth "google.golang.org/api/oauth2/v2"
	googleOption "google.golang.org/api/option"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/golang-jwt/jwt/v5"
)

const halfLifeDivisor = 2

type OAuthToken struct {
	Token     string
	Subject   string
	ExpiresOn time.Time
	RefreshOn time.Time
}

type OAuthTokenProvider interface {
	GetToken(ctx context.Context) (OAuthToken, error)
}

type GcpSubjectProvider interface {
	GetSubject(ctx context.Context, token string) (string, error)
}

type OAuthProviderOpts struct {
	azureTokenCredential azcore.TokenCredential
	gcpTokenProvider     gcpAuth.TokenProvider
	gcpSubjectProvider   GcpSubjectProvider
}

type OAuthTokenHandler interface {
	SetOAuthBearerToken(oauthBearerToken ckafka.OAuthBearerToken) error
	SetOAuthBearerTokenFailure(errstr string) error
}

func NewOAuthProvider(saslAlgorithm string, brokers []string, opts OAuthProviderOpts) (OAuthTokenProvider, error) {
	if saslAlgorithm == saslAzureEntra {
		return newAzureEntraOAuthTokenProvider(brokers, opts.azureTokenCredential)
	}

	if saslAlgorithm == saslGcpOauth {
		return newGcpOAuthTokenProvider(opts.gcpTokenProvider, opts.gcpSubjectProvider)
	}

	return nil, NewXk6KafkaError(failedCreateOAuthTokenProvider, saslAlgorithm+" is not a supported OAuth Provider.", nil)
}

type AzureEntraOAuthTokenProvider struct {
	tokenCredential azcore.TokenCredential
	requestOpts     policy.TokenRequestOptions
}

var _ OAuthTokenProvider = (*AzureEntraOAuthTokenProvider)(nil)

func newAzureEntraOAuthTokenProvider(
	brokers []string, tokenCredential azcore.TokenCredential,
) (*AzureEntraOAuthTokenProvider, error) {
	var cred azcore.TokenCredential
	var err error

	if len(brokers) != 1 {
		return nil, NewXk6KafkaError(
			failedGetOAuthToken,
			fmt.Sprintf("Azure Entra OAuth requires a single bootstrap server, but %d were provided.", len(brokers)),
			nil,
		)
	}

	host, _, err := net.SplitHostPort(brokers[0])
	if err != nil {
		return nil, NewXk6KafkaError(failedGetOAuthToken, "Azure Entra OAuth requires a valid host:port for the broker.", err)
	}

	scope := fmt.Sprintf("https://%s/.default", host)

	if tokenCredential == nil {
		cred, err = azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, NewXk6KafkaError(
				failedCreateOAuthTokenProvider,
				"Failed to create Azure Entra OAuth Token Provider.",
				err,
			)
		}
	} else {
		cred = tokenCredential
	}

	return &AzureEntraOAuthTokenProvider{
		tokenCredential: cred,
		requestOpts: policy.TokenRequestOptions{
			Scopes: []string{scope},
		},
	}, nil
}

func (a *AzureEntraOAuthTokenProvider) GetToken(ctx context.Context) (OAuthToken, error) {
	token, err := a.tokenCredential.GetToken(ctx, a.requestOpts)
	if err != nil {
		return OAuthToken{}, NewXk6KafkaError(failedGetOAuthToken, "Azure Entra OAuth token could not be retrieved.", err)
	}

	// The client retrieved this token and sends it to the server, which will verify it.
	// The token is only parsed to obtain the subject, since the Entra SDK does not include
	// the subject in the token struct.
	parsedToken, _, err := jwt.NewParser().ParseUnverified(token.Token, jwt.MapClaims{}) // NOSONAR
	if err != nil {
		return OAuthToken{}, NewXk6KafkaError(failedGetOAuthToken, "Failed to parse Azure Entra OAuth token.", err)
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return OAuthToken{}, NewXk6KafkaError(failedGetOAuthToken, "Azure Entra OAuth token contained invalid claims.", err)
	}

	subject, err := claims.GetSubject()
	if err != nil {
		return OAuthToken{}, NewXk6KafkaError(
			failedGetOAuthToken,
			"Azure Entra OAuth token is missing the subject claim.",
			err,
		)
	}

	return OAuthToken{
		Subject:   subject,
		Token:     token.Token,
		ExpiresOn: token.ExpiresOn,
		RefreshOn: token.RefreshOn,
	}, nil
}

var _ OAuthTokenProvider = (*GcpOAuthTokenProvider)(nil)

type GcpOAuthTokenProvider struct {
	tokenProvider   gcpAuth.TokenProvider
	subjectProvider GcpSubjectProvider
}

type GcpSdkSubjectProvider struct{}

var _ GcpSubjectProvider = (*GcpSdkSubjectProvider)(nil)

func (g *GcpSdkSubjectProvider) GetSubject(ctx context.Context, token string) (string, error) {
	oAuthService, err := googleOAuth.NewService(ctx, googleOption.WithoutAuthentication())
	if err != nil {
		return "", NewXk6KafkaError(failedGetOAuthToken, "Failed to configure GCP OAuth Service.", err)
	}

	tokenInfo, err := oAuthService.Tokeninfo().AccessToken(token).Do()
	if err != nil {
		return "", NewXk6KafkaError(failedGetOAuthToken, "Failed to get GCP OAuth Access Token information.", err)
	}

	return tokenInfo.Email, nil
}

func newGcpSdkSubjectProvider() (*GcpSdkSubjectProvider, error) {
	return &GcpSdkSubjectProvider{}, nil
}

func newGcpOAuthTokenProvider(
	tokenProvider gcpAuth.TokenProvider,
	subjectProvider GcpSubjectProvider,
) (*GcpOAuthTokenProvider, error) {
	var provider gcpAuth.TokenProvider
	var subProvider GcpSubjectProvider
	var err error

	if tokenProvider == nil {
		tokenProvider, err = gcpCredentials.DetectDefault(&gcpCredentials.DetectOptions{
			Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
		})
		if err != nil {
			return nil, NewXk6KafkaError(
				failedGetOAuthToken,
				"Failed to configure GCP OAuth Application Default Credentials (ADC).",
				err,
			)
		}

		provider = tokenProvider
	} else {
		provider = tokenProvider
	}

	if subjectProvider == nil {
		subjectProvider, err := newGcpSdkSubjectProvider()
		if err != nil {
			return nil, NewXk6KafkaError(failedGetOAuthToken, "Failed to Configure GCP OAuth Subject Provider.", err)
		}

		subProvider = subjectProvider
	} else {
		subProvider = subjectProvider
	}

	return &GcpOAuthTokenProvider{
		tokenProvider:   provider,
		subjectProvider: subProvider,
	}, nil
}

func (a *GcpOAuthTokenProvider) GetToken(ctx context.Context) (OAuthToken, error) {
	fetchTime := time.Now()

	token, err := a.tokenProvider.Token(ctx)
	if err != nil {
		return OAuthToken{}, NewXk6KafkaError(failedGetOAuthToken, "GCP OAuth token could not be retrieved.", err)
	}

	lifetime := token.Expiry.Sub(fetchTime)
	refreshOn := fetchTime.Add(lifetime / halfLifeDivisor)

	subject, err := a.subjectProvider.GetSubject(ctx, token.Value)
	if err != nil {
		return OAuthToken{}, NewXk6KafkaError(failedGetOAuthToken, "GCP OAuth token subject could not be retrieved.", err)
	}

	kafkaToken, err := buildGcpKafkaToken(token.Value, token.Expiry, subject)
	if err != nil {
		return OAuthToken{}, NewXk6KafkaError(failedGetOAuthToken, "Failed to build GCP Kafka OAuth token.", err)
	}

	return OAuthToken{
		Token:     kafkaToken,
		Subject:   subject,
		ExpiresOn: token.Expiry,
		RefreshOn: refreshOn,
	}, nil
}

func buildGcpKafkaToken(accessToken string, expiresOn time.Time, subject string) (string, error) {
	header := map[string]string{
		"typ": "JWT",
		"alg": "GOOG_OAUTH2_TOKEN",
	}
	headerBytes, err := json.Marshal(header)
	if err != nil {
		return "", err
	}

	payload := map[string]any{
		"exp":   expiresOn.Unix(),
		"iat":   time.Now().Unix(),
		"scope": "kafka",
		"sub":   subject,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerBytes)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadBytes)
	tokenB64 := base64.RawURLEncoding.EncodeToString([]byte(accessToken))

	return headerB64 + "." + payloadB64 + "." + tokenB64, nil
}

func refreshOAuthToken(ctx context.Context, saslContext SASLContext, handler OAuthTokenHandler) error {
	if saslContext.OAuthProvider == nil {
		return NewXk6KafkaError(
			failedGetOAuthToken,
			"Failed to get an OAuth token. The OAuth provider is nil in the SASL context.",
			nil,
		)
	}

	oauthProvider := *saslContext.OAuthProvider

	token, err := oauthProvider.GetToken(ctx)
	if err == nil {
		err = handler.SetOAuthBearerToken(ckafka.OAuthBearerToken{
			TokenValue: token.Token,
			Expiration: token.ExpiresOn,
			Principal:  token.Subject,
			Extensions: make(map[string]string),
		})
		if err != nil {
			return NewXk6KafkaError(
				failedGetOAuthToken,
				"Failed to set an OAuth token. The Kafka client rejected the OAuth token.",
				err,
			)
		}
	} else {
		err = handler.SetOAuthBearerTokenFailure(err.Error())
		if err != nil {
			return NewXk6KafkaError(
				failedGetOAuthToken,
				"Failed to set an OAuth token error. The Kafka client rejected the OAuth token error.",
				err,
			)
		}
	}
	return nil
}
