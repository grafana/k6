package kafka

import (
	"errors"
	"fmt"

	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

// errAWSIAMNotImplemented is returned for SASL_AWS_IAM, which is declared in the
// contract but deferred to a dedicated change (it needs an AWS credential
// provider and pulls in the AWS SDK).
var errAWSIAMNotImplemented = errors.New("SASL_AWS_IAM is not yet implemented")

// buildSASL selects the franz-go SASL mechanism for the configured algorithm.
// It returns (nil, nil) when no SASL is configured. `sasl_ssl` uses PLAIN and
// requires TLS to be enabled (matching v1 behavior).
func buildSASL(sc *SASLConfig, tlsEnabled bool) (sasl.Mechanism, error) {
	if sc == nil || sc.Algorithm == "" || sc.Algorithm == saslNone {
		return nil, nil //nolint:nilnil // a nil mechanism means "no SASL", not an error
	}

	switch sc.Algorithm {
	case saslPlain:
		return plain.Auth{User: sc.Username, Pass: sc.Password}.AsMechanism(), nil
	case saslSsl:
		if !tlsEnabled {
			return nil, errors.New("sasl_ssl requires TLS to be enabled")
		}
		return plain.Auth{User: sc.Username, Pass: sc.Password}.AsMechanism(), nil
	case saslScramSha256:
		return scram.Auth{User: sc.Username, Pass: sc.Password}.AsSha256Mechanism(), nil
	case saslScramSha512:
		return scram.Auth{User: sc.Username, Pass: sc.Password}.AsSha512Mechanism(), nil
	case saslAwsIam:
		return nil, errAWSIAMNotImplemented
	default:
		return nil, fmt.Errorf("unknown SASL algorithm %q", sc.Algorithm)
	}
}
