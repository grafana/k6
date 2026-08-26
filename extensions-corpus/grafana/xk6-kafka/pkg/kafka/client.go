package kafka

import "github.com/twmb/franz-go/pkg/kgo"

// clientOptions builds the franz-go client options shared by all classes from
// broker addresses plus optional SASL and TLS configuration.
func clientOptions(brokers []string, sc *SASLConfig, tc *TLSConfig) ([]kgo.Opt, error) {
	opts := []kgo.Opt{kgo.SeedBrokers(brokers...)}

	tlsCfg, err := buildTLSConfig(tc)
	if err != nil {
		return nil, err
	}
	if tlsCfg != nil {
		opts = append(opts, kgo.DialTLSConfig(tlsCfg))
	}

	mech, err := buildSASL(sc, tlsCfg != nil)
	if err != nil {
		return nil, err
	}
	if mech != nil {
		opts = append(opts, kgo.SASL(mech))
	}

	return opts, nil
}
