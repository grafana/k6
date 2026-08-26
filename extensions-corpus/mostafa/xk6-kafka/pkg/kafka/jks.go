package kafka

import (
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"os"

	"github.com/grafana/sobek"
	"github.com/pavlo-v-chernykh/keystore-go/v4"
	"go.k6.io/k6/js/common"
)

const (
	// filePermission is the permission used for writing certificate and key files.
	filePermission = 0o600
)

type JKSConfig struct {
	Path              string `json:"path"`
	Password          string `json:"password"`
	ClientCertAlias   string `json:"clientCertAlias"`
	ClientKeyAlias    string `json:"clientKeyAlias"`
	ClientKeyPassword string `json:"clientKeyPassword"`
	ServerCaAlias     string `json:"serverCaAlias"`
}

type JKS struct {
	ClientCertsPem []string `json:"clientCertsPem"`
	ClientKeyPem   string   `json:"clientKeyPem"`
	ServerCaPem    string   `json:"serverCaPem"`
}

func (*Kafka) loadJKS(jksConfig *JKSConfig) (*JKS, *Xk6KafkaError) {
	if jksConfig == nil {
		return nil, nil
	}

	fileErr := fileExists(jksConfig.Path)
	if fileErr != nil {
		return nil, NewXk6KafkaError(
			fileNotFound, "File not found: "+jksConfig.Path, fileErr)
	}

	jksFile, err := os.Open(jksConfig.Path)
	if err != nil {
		return nil, NewXk6KafkaError(
			failedReadJKSFile, "Failed to read JKS file: "+jksConfig.Path, err)
	}

	ks := keystore.New()
	if err := ks.Load(jksFile, []byte(jksConfig.Password)); err != nil {
		return nil, NewXk6KafkaError(
			failedDecodeJKSFile, "Failed to decode JKS file: "+jksConfig.Path, err)
	}

	// Load server's CA certificate if an alias is provided.
	// This makes it possible to use mTLS using two separate
	// JKS files for loading the certificates:
	// one for the client and one for the server.
	var serverCa keystore.TrustedCertificateEntry
	if jksConfig.ServerCaAlias != "" {
		serverCa, err = ks.GetTrustedCertificateEntry(jksConfig.ServerCaAlias)
		if err != nil {
			return nil, NewXk6KafkaError(
				failedDecodeServerCa,
				"Failed to decode server's CA: "+jksConfig.Path, err)
		}
	} else {
		serverCa = keystore.TrustedCertificateEntry{}
		// Log a message if no server's CA alias is provided.
		log.Println("No server's CA alias provided, skipping server's CA")
	}

	// Load client's certificate and key if aliases are provided.
	clientKey, err := ks.GetPrivateKeyEntry(
		jksConfig.ClientKeyAlias, []byte(jksConfig.ClientKeyPassword))
	if err != nil {
		// Server CA is loaded, so it will be saved, even though the client key is not loaded.
		// This is because one might not have enabled mutual TLS (mTLS).
		serverCaFilename := "server-ca.pem"
		if serverCa.Certificate.Content != nil {
			if err := saveServerCaFile(serverCaFilename, &serverCa); err != nil {
				return nil, err
			}
		}

		return &JKS{
				ClientCertsPem: nil,
				ClientKeyPem:   "",
				ServerCaPem:    serverCaFilename,
			}, NewXk6KafkaError(
				failedDecodePrivateKey,
				"Failed to decode client's private key: "+jksConfig.Path, err)
	}

	clientCertsFilenames := make([]string, 0, len(clientKey.CertificateChain))
	for idx, cert := range clientKey.CertificateChain {
		filename := fmt.Sprintf("client-cert-%d.pem", idx)
		if err := saveClientCertFile(filename, &cert); err != nil {
			return nil, err
		}
		clientCertsFilenames = append(clientCertsFilenames, filename)
	}

	clientKeyFilename := "client-key.pem"
	if err := saveClientKeyFile(clientKeyFilename, &clientKey); err != nil {
		return nil, err
	}

	serverCaFilename := "server-ca.pem"
	if serverCa.Certificate.Content != nil {
		if err := saveServerCaFile(serverCaFilename, &serverCa); err != nil {
			return nil, err
		}
	}

	return &JKS{
		ClientCertsPem: clientCertsFilenames,
		ClientKeyPem:   clientKeyFilename,
		ServerCaPem:    serverCaFilename,
	}, nil
}

func (k *Kafka) loadJKSFunction(call sobek.FunctionCall) sobek.Value {
	runtime := k.vu.Runtime()
	var jksConfig *JKSConfig

	if len(call.Arguments) == 0 {
		common.Throw(runtime, ErrNotEnoughArguments)
	}

	if params, ok := call.Argument(0).Export().(map[string]any); ok {
		if b, err := json.Marshal(params); err != nil {
			common.Throw(runtime, err)
		} else {
			if err = json.Unmarshal(b, &jksConfig); err != nil {
				common.Throw(runtime, err)
			}
		}
	}

	if jksConfig == nil {
		common.Throw(runtime, ErrNoJKSConfig)
	}

	jks, err := k.loadJKS(jksConfig)
	if jks == nil && err != nil {
		common.Throw(runtime, err)
	}

	obj := runtime.NewObject()
	// Zero-value fields are not returned to JS.
	if jks.ServerCaPem != "" {
		if err := obj.Set("serverCaPem", jks.ServerCaPem); err != nil {
			common.Throw(runtime, err)
		}
	}
	if jks.ClientCertsPem != nil {
		if err := obj.Set("clientCertsPem", jks.ClientCertsPem); err != nil {
			common.Throw(runtime, err)
		}
	}
	if jks.ClientKeyPem != "" {
		if err := obj.Set("clientKeyPem", jks.ClientKeyPem); err != nil {
			common.Throw(runtime, err)
		}
	}
	return runtime.ToValue(obj)
}

// saveServerCaFile saves the server's CA certificate to a file.
func saveServerCaFile(filename string, cert *keystore.TrustedCertificateEntry) *Xk6KafkaError {
	certPem := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Certificate.Content,
	})

	if err := os.WriteFile(filename, certPem, filePermission); err != nil {
		return NewXk6KafkaError(
			failedWriteServerCaFile, "Failed to write CA file", err)
	}

	return nil
}

// saveClientKeyFile saves the client's key to a file.
func saveClientKeyFile(filename string, key *keystore.PrivateKeyEntry) *Xk6KafkaError {
	keyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: key.PrivateKey,
	})

	if err := os.WriteFile(filename, keyPem, filePermission); err != nil {
		return NewXk6KafkaError(
			failedWriteKeyFile, "Failed to write key file", err)
	}

	return nil
}

// saveClientCertFile saves the client's certificate to a file.
func saveClientCertFile(filename string, cert *keystore.Certificate) *Xk6KafkaError {
	clientCertPem := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Content,
	})
	if err := os.WriteFile(filename, clientCertPem, filePermission); err != nil {
		return NewXk6KafkaError(
			failedWriteCertFile, "Failed to write cert file", err)
	}

	return nil
}
