package kafka

import (
	"context"
	"testing"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureContext(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	assert.Equal(t, ctx, ensureContext(ctx))
	//nolint:staticcheck // explicit nil exercises fallback to context.Background()
	bg := ensureContext(nil)
	assert.NotNil(t, bg)
	assert.NoError(t, bg.Err())
}

func TestCloneConfluentConfigMap(t *testing.T) {
	t.Parallel()
	src := ckafka.ConfigMap{"a": "1", "b": 2}
	dst := cloneConfluentConfigMap(src)
	assert.Equal(t, src, dst)
	src["a"] = "changed"
	assert.Equal(t, "1", dst["a"])
}

func TestNewConfluentConfigMap(t *testing.T) {
	t.Parallel()
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		cm, err := newConfluentConfigMap([]string{"a:1", "b:2"})
		require.NoError(t, err)
		assert.Equal(t, "a:1,b:2", cm["bootstrap.servers"])
	})
	t.Run("empty brokers", func(t *testing.T) {
		t.Parallel()
		_, err := newConfluentConfigMap(nil)
		require.Error(t, err)
		var xk *Xk6KafkaError
		require.ErrorAs(t, err, &xk)
	})
}

func TestApplyConfluentSecurityConfig(t *testing.T) {
	t.Parallel()
	newMap := func() ckafka.ConfigMap {
		m := ckafka.ConfigMap{}
		require.NoError(t, setConfluentConfigValue(m, "bootstrap.servers", "localhost:9092"))
		return m
	}

	t.Run("plaintext no sasl", func(t *testing.T) {
		t.Parallel()
		cfg := newMap()
		require.NoError(t, applyConfluentSecurityConfig(cfg, SASLConfig{}, TLSConfig{}))
		assert.Equal(t, "PLAINTEXT", cfg["security.protocol"])
	})

	t.Run("ssl only", func(t *testing.T) {
		t.Parallel()
		cfg := newMap()
		require.NoError(t, applyConfluentSecurityConfig(cfg, SASLConfig{}, TLSConfig{EnableTLS: true}))
		assert.Equal(t, "SSL", cfg["security.protocol"])
	})

	t.Run("sasl ssl without tls error", func(t *testing.T) {
		t.Parallel()
		cfg := newMap()
		err := applyConfluentSecurityConfig(cfg, SASLConfig{Algorithm: saslSsl}, TLSConfig{})
		require.Error(t, err)
	})

	t.Run("sasl plain with tls", func(t *testing.T) {
		t.Parallel()
		cfg := newMap()
		require.NoError(t, applyConfluentSecurityConfig(cfg, SASLConfig{
			Algorithm: saslPlain,
			Username:  "u",
			Password:  "p",
		}, TLSConfig{EnableTLS: true}))
		assert.Equal(t, "SASL_SSL", cfg["security.protocol"])
		assert.Equal(t, "PLAIN", cfg["sasl.mechanism"])
		assert.Equal(t, "u", cfg["sasl.username"])
	})

	t.Run("sasl plain without tls", func(t *testing.T) {
		t.Parallel()
		cfg := newMap()
		require.NoError(t, applyConfluentSecurityConfig(cfg, SASLConfig{
			Algorithm: saslPlain,
			Username:  "u",
			Password:  "p",
		}, TLSConfig{}))
		assert.Equal(t, "SASL_PLAINTEXT", cfg["security.protocol"])
	})

	t.Run("sasl gssapi", func(t *testing.T) {
		t.Parallel()
		cfg := newMap()
		require.NoError(t, applyConfluentSecurityConfig(cfg, SASLConfig{Algorithm: saslGssApi}, TLSConfig{EnableTLS: true}))
		assert.Equal(t, "SASL_SSL", cfg["security.protocol"])
		assert.Equal(t, "GSSAPI", cfg["sasl.mechanism"])
	})

	t.Run("sasl gssapi with kerberos config", func(t *testing.T) {
		t.Parallel()
		cfg := newMap()
		require.NoError(t, applyConfluentSecurityConfig(
			cfg,
			SASLConfig{
				Algorithm: saslGssApi,
				KerberosConfig: KerberosConfig{
					ServiceName:          new("kafkaTest"),
					Principal:            new("test@realm"),
					KInitCmd:             new("/path/to/kinit"),
					KeyTab:               new("/path/to/test/keytab"),
					MinTimeBeforeRelogin: new(1000),
				},
			},
			TLSConfig{EnableTLS: true},
		))
		assert.Equal(t, "SASL_SSL", cfg["security.protocol"])
		assert.Equal(t, "GSSAPI", cfg["sasl.mechanism"])
		assert.Equal(t, "kafkaTest", cfg["sasl.kerberos.service.name"])
		assert.Equal(t, "test@realm", cfg["sasl.kerberos.principal"])
		assert.Equal(t, "/path/to/kinit", cfg["sasl.kerberos.kinit.cmd"])
		assert.Equal(t, "/path/to/test/keytab", cfg["sasl.kerberos.keytab"])
		assert.Equal(t, 1000, cfg["sasl.kerberos.min.time.before.relogin"])
	})

	t.Run("sasl gssapi with kerberos default config", func(t *testing.T) {
		t.Parallel()
		cfg := newMap()
		require.NoError(t, applyConfluentSecurityConfig(cfg, SASLConfig{
			Algorithm:      saslGssApi,
			KerberosConfig: KerberosConfig{},
		}, TLSConfig{EnableTLS: true}))
		assert.Equal(t, "SASL_SSL", cfg["security.protocol"])
		assert.Equal(t, "GSSAPI", cfg["sasl.mechanism"])
		assert.Equal(t, "SASL_SSL", cfg["security.protocol"])
		assert.Nil(t, cfg["sasl.kerberos.service.name"])
		assert.Nil(t, cfg["sasl.kerberos.principal"])
		assert.Nil(t, cfg["sasl.kerberos.kinit.cmd"])
		assert.Nil(t, cfg["sasl.kerberos.keytab"])
		assert.Nil(t, cfg["sasl.kerberos.min.time.before.relogin"])
	})

	t.Run("scram sha512", func(t *testing.T) {
		t.Parallel()
		cfg := newMap()
		require.NoError(t, applyConfluentSecurityConfig(cfg, SASLConfig{
			Algorithm: saslScramSha512,
			Username:  "u",
			Password:  "p",
		}, TLSConfig{EnableTLS: true}))
		assert.Equal(t, "SCRAM-SHA-512", cfg["sasl.mechanism"])
	})

	t.Run("tls pem fields", func(t *testing.T) {
		t.Parallel()
		cfg := newMap()
		require.NoError(t, applyConfluentSecurityConfig(cfg, SASLConfig{}, TLSConfig{
			EnableTLS:             true,
			ServerCaPem:           "ca",
			ClientCertPem:         "cert",
			ClientKeyPem:          "key",
			InsecureSkipTLSVerify: true,
		}))
		assert.Equal(t, "ca", cfg["ssl.ca.pem"])
		assert.Equal(t, "cert", cfg["ssl.certificate.pem"])
		assert.Equal(t, "key", cfg["ssl.key.pem"])
		assert.Equal(t, false, cfg["enable.ssl.certificate.verification"])
	})

	t.Run("sasl ssl with tls and plain credentials", func(t *testing.T) {
		t.Parallel()
		cfg := newMap()
		require.NoError(t, applyConfluentSecurityConfig(cfg, SASLConfig{
			Algorithm: saslSsl,
			Username:  "u",
			Password:  "p",
		}, TLSConfig{EnableTLS: true}))
		assert.Equal(t, "SASL_SSL", cfg["security.protocol"])
		assert.Equal(t, "PLAIN", cfg["sasl.mechanism"])
		assert.Equal(t, "u", cfg["sasl.username"])
		assert.Equal(t, "p", cfg["sasl.password"])
	})

	t.Run("azure entra", func(t *testing.T) {
		t.Parallel()
		cfg := newMap()
		require.NoError(t, applyConfluentSecurityConfig(cfg, SASLConfig{
			Algorithm: saslAzureEntra,
		}, TLSConfig{EnableTLS: true}))
		assert.Equal(t, "SASL_SSL", cfg["security.protocol"])
		assert.Equal(t, "OAUTHBEARER", cfg["sasl.mechanism"])
	})

	t.Run("aws iam not wired", func(t *testing.T) {
		t.Parallel()
		cfg := newMap()
		err := applyConfluentSecurityConfig(cfg, SASLConfig{Algorithm: saslAwsIam}, TLSConfig{EnableTLS: true})
		require.Error(t, err)
	})

	t.Run("gcp oauth", func(t *testing.T) {
		t.Parallel()
		cfg := newMap()
		require.NoError(t, applyConfluentSecurityConfig(cfg, SASLConfig{
			Algorithm: saslGcpOauth,
		}, TLSConfig{EnableTLS: true}))
		assert.Equal(t, "SASL_SSL", cfg["security.protocol"])
		assert.Equal(t, "OAUTHBEARER", cfg["sasl.mechanism"])
	})

	t.Run("unsupported security protocol", func(t *testing.T) {
		t.Parallel()
		cfg := newMap()
		err := applyConfluentSecurityConfig(cfg, SASLConfig{Algorithm: "unknown_mechanism"}, TLSConfig{})
		require.Error(t, err)
	})
}

func TestConfluentSecurityProtocolAndMechanism(t *testing.T) {
	t.Parallel()
	t.Run("unsupported algorithm", func(t *testing.T) {
		t.Parallel()
		_, err := confluentSecurityProtocol(SASLConfig{Algorithm: "bogus"}, TLSConfig{})
		require.Error(t, err)
	})
	t.Run("unsupported mechanism only", func(t *testing.T) {
		t.Parallel()
		_, err := confluentSASLMechanism(SASLConfig{Algorithm: "bogus"})
		require.Error(t, err)
	})
	t.Run("aws iam mechanism string", func(t *testing.T) {
		t.Parallel()
		m, err := confluentSASLMechanism(SASLConfig{Algorithm: saslAwsIam})
		require.NoError(t, err)
		assert.Equal(t, "AWS_MSK_IAM", m)
	})
	t.Run("aws iam protocol with tls", func(t *testing.T) {
		t.Parallel()
		p, err := confluentSecurityProtocol(SASLConfig{Algorithm: saslAwsIam}, TLSConfig{EnableTLS: true})
		require.NoError(t, err)
		assert.Equal(t, "SASL_SSL", p)
	})
	t.Run("aws iam protocol without tls", func(t *testing.T) {
		t.Parallel()
		p, err := confluentSecurityProtocol(SASLConfig{Algorithm: saslAwsIam}, TLSConfig{})
		require.NoError(t, err)
		assert.Equal(t, "SASL_PLAINTEXT", p)
	})
}

func TestWriterConfigToConfluentConfigMap_Table(t *testing.T) {
	t.Parallel()
	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		_, err := writerConfigToConfluentConfigMap(nil)
		require.Error(t, err)
	})
	t.Run("optional fields", func(t *testing.T) {
		t.Parallel()
		cfg, err := writerConfigToConfluentConfigMap(&WriterConfig{
			Brokers:      []string{"localhost:9092"},
			WriteTimeout: 2 * time.Second,
			ReadTimeout:  3 * time.Second,
			RequiredAcks: 1,
			BatchTimeout: 100 * time.Millisecond,
		})
		require.NoError(t, err)
		assert.Equal(t, 2000, cfg["message.timeout.ms"])
		assert.Equal(t, 3000, cfg["socket.timeout.ms"])
		assert.Equal(t, "1", cfg["acks"])
		assert.Equal(t, 100, cfg["linger.ms"])
	})
	t.Run("invalid required acks", func(t *testing.T) {
		t.Parallel()
		_, err := writerConfigToConfluentConfigMap(&WriterConfig{
			Brokers:      []string{"localhost:9092"},
			RequiredAcks: 99,
		})
		require.Error(t, err)
	})
	t.Run("explicit zero acks and compression", func(t *testing.T) {
		t.Parallel()
		cfg, err := writerConfigToConfluentConfigMap(&WriterConfig{
			Brokers:      []string{"localhost:9092"},
			RequiredAcks: 0,
			BatchSize:    100,
			BatchBytes:   4096,
			Compression:  codecSnappy,
		})
		require.NoError(t, err)
		assert.Equal(t, "0", cfg["acks"])
		assert.Equal(t, 100, cfg["batch.num.messages"])
		assert.Equal(t, 4096, cfg["batch.size"])
		assert.Equal(t, "snappy", cfg["compression.type"])
	})
}

func TestReaderConfigToConfluentConfigMap_Table(t *testing.T) {
	t.Parallel()
	base := ReaderConfig{
		Brokers:           []string{"localhost:9092"},
		GroupID:           "g",
		MinBytes:          1,
		MaxBytes:          1 << 20,
		MaxWait:           Duration{Duration: 500 * time.Millisecond},
		SessionTimeout:    10 * time.Second,
		HeartbeatInterval: 3 * time.Second,
		CommitInterval:    5 * time.Second,
	}

	t.Run("first offset reset", func(t *testing.T) {
		t.Parallel()
		rc := base
		rc.StartOffset = firstOffset
		cfg, err := readerConfigToConfluentConfigMap(&rc)
		require.NoError(t, err)
		assert.Equal(t, "earliest", cfg["auto.offset.reset"])
	})
	t.Run("last offset reset", func(t *testing.T) {
		t.Parallel()
		rc := base
		rc.StartOffset = lastOffset
		cfg, err := readerConfigToConfluentConfigMap(&rc)
		require.NoError(t, err)
		assert.Equal(t, "latest", cfg["auto.offset.reset"])
	})
	t.Run("numeric start offset uses earliest", func(t *testing.T) {
		t.Parallel()
		rc := base
		rc.StartOffset = "42"
		cfg, err := readerConfigToConfluentConfigMap(&rc)
		require.NoError(t, err)
		assert.Equal(t, "earliest", cfg["auto.offset.reset"])
	})
	t.Run("invalid numeric start offset still earliest", func(t *testing.T) {
		t.Parallel()
		rc := base
		rc.StartOffset = "not-a-number"
		cfg, err := readerConfigToConfluentConfigMap(&rc)
		require.NoError(t, err)
		assert.Equal(t, "earliest", cfg["auto.offset.reset"])
	})
	t.Run("empty start offset same as first", func(t *testing.T) {
		t.Parallel()
		rc := base
		rc.StartOffset = ""
		cfg, err := readerConfigToConfluentConfigMap(&rc)
		require.NoError(t, err)
		assert.Equal(t, "earliest", cfg["auto.offset.reset"])
	})
	t.Run("nil reader config", func(t *testing.T) {
		t.Parallel()
		_, err := readerConfigToConfluentConfigMap(nil)
		require.Error(t, err)
	})
	t.Run("without group id sets fetch tuning only", func(t *testing.T) {
		t.Parallel()
		cfg, err := readerConfigToConfluentConfigMap(&ReaderConfig{
			Brokers:  []string{"localhost:9092"},
			GroupID:  "",
			Topic:    "t",
			MinBytes: 128,
			MaxBytes: 2 << 20,
			MaxWait:  Duration{Duration: 2 * time.Second},
		})
		require.NoError(t, err)
		assert.Equal(t, 128, cfg["fetch.min.bytes"])
		assert.Equal(t, 2<<20, cfg["fetch.max.bytes"])
		assert.Equal(t, 2000, cfg["fetch.wait.max.ms"])
		_, hasGroup := cfg["group.id"]
		assert.False(t, hasGroup)
	})
}

func TestConnectionConfigToConfluentConfigMap_Table(t *testing.T) {
	t.Parallel()
	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		_, err := connectionConfigToConfluentConfigMap(nil)
		require.Error(t, err)
	})
	t.Run("address fallback", func(t *testing.T) {
		t.Parallel()
		cfg, err := connectionConfigToConfluentConfigMap(&ConnectionConfig{
			Address: "localhost:9092",
		})
		require.NoError(t, err)
		assert.Equal(t, "localhost:9092", cfg["bootstrap.servers"])
	})
	t.Run("brokers preferred over address", func(t *testing.T) {
		t.Parallel()
		cfg, err := connectionConfigToConfluentConfigMap(&ConnectionConfig{
			Address: "ignored:9092",
			Brokers: []string{"a:1", "b:2"},
		})
		require.NoError(t, err)
		assert.Equal(t, "a:1,b:2", cfg["bootstrap.servers"])
	})
	t.Run("empty brokers and address", func(t *testing.T) {
		t.Parallel()
		_, err := connectionConfigToConfluentConfigMap(&ConnectionConfig{})
		require.Error(t, err)
	})
}

func TestConfluentOffset(t *testing.T) {
	t.Parallel()
	t.Run("explicit offset wins", func(t *testing.T) {
		t.Parallel()
		o, err := confluentOffset("garbage", 99)
		require.NoError(t, err)
		assert.Equal(t, ckafka.Offset(99), o)
	})
	t.Run("beginning", func(t *testing.T) {
		t.Parallel()
		o, err := confluentOffset(firstOffset, 0)
		require.NoError(t, err)
		assert.Equal(t, ckafka.OffsetBeginning, o)
	})
	t.Run("end", func(t *testing.T) {
		t.Parallel()
		o, err := confluentOffset(lastOffset, 0)
		require.NoError(t, err)
		assert.Equal(t, ckafka.OffsetEnd, o)
	})
	t.Run("numeric string", func(t *testing.T) {
		t.Parallel()
		o, err := confluentOffset("7", 0)
		require.NoError(t, err)
		assert.Equal(t, ckafka.Offset(7), o)
	})
	t.Run("invalid string", func(t *testing.T) {
		t.Parallel()
		_, err := confluentOffset("x", 0)
		require.Error(t, err)
	})
}

func TestConfluentPollTimeout(t *testing.T) {
	t.Parallel()
	t.Run("nil context uses default step", func(t *testing.T) {
		t.Parallel()
		//nolint:staticcheck // explicit nil exercises default poll step
		d := confluentPollTimeout(nil)
		assert.Equal(t, 100*time.Millisecond, d)
	})
	t.Run("deadline exceeded", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()
		assert.Equal(t, time.Duration(0), confluentPollTimeout(ctx))
	})
	t.Run("short remaining", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(30*time.Millisecond))
		defer cancel()
		d := confluentPollTimeout(ctx)
		assert.Greater(t, d, time.Duration(0))
		assert.LessOrEqual(t, d, 30*time.Millisecond)
	})
}

func TestConfluentMetadataTimeoutMs(t *testing.T) {
	t.Parallel()
	t.Run("default without deadline", func(t *testing.T) {
		t.Parallel()
		//nolint:staticcheck // explicit nil exercises default metadata timeout
		assert.Equal(t, 5000, confluentMetadataTimeoutMs(nil))
		assert.Equal(t, 5000, confluentMetadataTimeoutMs(context.Background()))
	})
	t.Run("past deadline", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()
		assert.Equal(t, 0, confluentMetadataTimeoutMs(ctx))
	})
	t.Run("with deadline", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(750*time.Millisecond))
		defer cancel()
		ms := confluentMetadataTimeoutMs(ctx)
		assert.Greater(t, ms, 0)
		assert.LessOrEqual(t, ms, 750)
	})
}
