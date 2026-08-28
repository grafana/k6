package kafka

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/twmb/franz-go/pkg/kgo"
	extensionapi "go.k6.io/k6-extension-api"
)

// clientOptions builds the franz-go client options shared by all classes from
// broker addresses plus optional SASL and TLS configuration.
func clientOptions(vu extensionapi.VU, brokers []string, sc *SASLConfig, tc *TLSConfig) ([]kgo.Opt, error) {
	opts := []kgo.Opt{kgo.SeedBrokers(brokers...)}

	tlsCfg, err := buildTLSConfig(tc)
	if err != nil {
		return nil, err
	}
	opts = append(opts, kgo.Dialer(kafkaDialer(vu, tlsCfg)))

	mech, err := buildSASL(sc, tlsCfg != nil)
	if err != nil {
		return nil, err
	}
	if mech != nil {
		opts = append(opts, kgo.SASL(mech))
	}

	return opts, nil
}

func kafkaDialer(vu extensionapi.VU, tlsConfig *tls.Config) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		networkCapability, ok := vu.(extensionapi.Network)
		if !ok {
			return nil, extensionapi.ErrNetworkUnavailable
		}
		connection, err := networkCapability.DialContext(ctx, network, address)
		if err != nil || tlsConfig == nil {
			return connection, err
		}
		tlsCapability, ok := vu.(extensionapi.TLS)
		if !ok {
			_ = connection.Close()
			return nil, extensionapi.ErrTLSUnavailable
		}

		config := tlsConfig.Clone()
		if config.ServerName == "" {
			serverName, _, splitErr := net.SplitHostPort(address)
			if splitErr != nil {
				_ = connection.Close()
				return nil, fmt.Errorf("split Kafka broker address: %w", splitErr)
			}
			config.ServerName = serverName
		}
		return tlsCapability.TLSClient(ctx, connection, config)
	}
}
