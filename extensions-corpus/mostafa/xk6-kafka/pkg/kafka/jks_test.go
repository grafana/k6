package kafka

import (
	"os"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
)

type SimpleJKSConfig struct {
	jksConfig JKSConfig
	err       *Xk6KafkaError
}

func TestJKS(t *testing.T) {
	jksConfigs := []*SimpleJKSConfig{
		{
			jksConfig: JKSConfig{
				Path:              "testdata/fixtures/kafka-keystore.jks",
				Password:          "password",
				ClientCertAlias:   "localhost",
				ClientKeyAlias:    "localhost",
				ClientKeyPassword: "password",
				ServerCaAlias:     "caroot",
			},
			err: nil,
		},
		{
			jksConfig: JKSConfig{
				Path: "testdata/fixtures/kafka-keystore-not-exists.jks",
			},
			err: NewXk6KafkaError(
				fileNotFound, "File not found: testdata/fixtures/kafka-keystore-not-exists.jks", nil),
		},
		{
			jksConfig: JKSConfig{
				Path:     "testdata/fixtures/kafka-truststore.jks",
				Password: "wrong-password",
			},
			err: NewXk6KafkaError(
				failedDecodeJKSFile, "Failed to decode JKS file: testdata/fixtures/kafka-truststore.jks", nil),
		},
		{
			jksConfig: JKSConfig{
				Path:     "testdata/fixtures/kafka-truststore.jks",
				Password: "password",
			},
			err: NewXk6KafkaError(
				failedDecodePrivateKey, "Failed to decode client's private key: testdata/fixtures/kafka-truststore.jks", nil),
		},
		{
			jksConfig: JKSConfig{
				Path:          "testdata/fixtures/kafka-keystore.jks",
				Password:      "password",
				ServerCaAlias: "wrong-alias", // This alias does not exist in the JKS file.
			},
			err: NewXk6KafkaError(
				failedDecodeServerCa, "Failed to decode server's CA: testdata/fixtures/kafka-keystore.jks", nil),
		},
		{
			jksConfig: JKSConfig{
				Path:              "testdata/fixtures/kafka-keystore.jks",
				Password:          "password",
				ServerCaAlias:     "caroot",
				ClientKeyAlias:    "wrong-alias", // These alias don't exist in the JKS file.
				ClientCertAlias:   "wrong-alias",
				ClientKeyPassword: "password", // This password is correct.
			},
			err: NewXk6KafkaError(
				failedDecodePrivateKey, "Failed to decode client's private key: testdata/fixtures/kafka-keystore.jks", nil),
		},
		{
			jksConfig: JKSConfig{
				Path:              "testdata/fixtures/kafka-keystore.jks",
				Password:          "password",
				ClientCertAlias:   "localhost",
				ClientKeyAlias:    "localhost",
				ClientKeyPassword: "wrong-password",
				ServerCaAlias:     "caroot",
			},
			err: NewXk6KafkaError(
				failedDecodePrivateKey, "Failed to decode client's private key: testdata/fixtures/kafka-keystore.jks", nil),
		},
	}

	k := &Kafka{}

	for _, jksConfig := range jksConfigs {
		jks, err := k.loadJKS(&jksConfig.jksConfig)
		if jksConfig.err != nil {
			assert.Equal(t, jksConfig.err.Code, err.Code)
			assert.Equal(t, jksConfig.err.Message, err.Message)

			if jksConfig.err.Code == failedDecodePrivateKey {
				assert.NotNil(t, jks)
				assert.Nil(t, jks.ClientCertsPem)
				assert.Empty(t, jks.ClientCertsPem)
				assert.Equal(t, jks.ClientKeyPem, "")
				assert.Empty(t, jks.ClientKeyPem)
				assert.NotNil(t, jks.ServerCaPem)
				assert.NotEmpty(t, jks.ServerCaPem)
			}
			continue
		}
		assert.Nil(t, err)
		assert.NotNil(t, jks)

		assert.NotNil(t, jks.ClientCertsPem)
		assert.NotEmpty(t, jks.ClientCertsPem)
		assert.NotNil(t, jks.ClientKeyPem)
		assert.NotEmpty(t, jks.ClientKeyPem)
		assert.NotNil(t, jks.ServerCaPem)
		assert.NotEmpty(t, jks.ServerCaPem)
	}
}

func TestLoadJKS_Function(t *testing.T) {
	test := getTestModuleInstance(t)

	jks := test.module.loadJKSFunction(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.module.vu.Runtime().ToValue(
				map[string]any{
					"path":              "testdata/fixtures/kafka-keystore.jks",
					"password":          "password",
					"clientCertAlias":   "localhost",
					"clientKeyAlias":    "localhost",
					"clientKeyPassword": "password",
					"serverCaAlias":     "caroot",
				},
			),
		},
	})

	assert.NotNil(t, jks)

	jksMap := jks.ToObject(test.vu.Runtime())
	assert.NotNil(t, jksMap.Get("clientCertsPem"))
	assert.NotNil(t, jksMap.Get("clientKeyPem"))
	assert.NotNil(t, jksMap.Get("serverCaPem"))

	// Remove generated files after the test.
	assert.NoError(t, os.Remove("client-cert-0.pem"))
	assert.NoError(t, os.Remove("client-cert-1.pem"))
	assert.NoError(t, os.Remove("client-key.pem"))
	assert.NoError(t, os.Remove("server-ca.pem"))
}
