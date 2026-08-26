package kafka

import (
	"bytes"
	"encoding/pem"
	"errors"
	"fmt"
	"io"

	"github.com/grafana/sobek"
	keystore "github.com/pavlo-v-chernykh/keystore-go/v4"
	"go.k6.io/k6/v2/js/common"
)

// loadJKS implements the LoadJKS function. It is an init-context operation: it
// reads the keystore through k6's init-environment filesystem (archive-aware),
// not the host OS, and returns PEM material.
func (m *Module) loadJKS(call sobek.FunctionCall) sobek.Value {
	rt := m.vu.Runtime()

	initEnv := m.vu.InitEnv()
	if initEnv == nil {
		common.Throw(rt, errors.New("LoadJKS must be called in the init context"))
	}

	var cfg JKSConfig
	if err := rt.ExportTo(call.Argument(0), &cfg); err != nil {
		common.Throw(rt, fmt.Errorf("invalid JKS config: %w", err))
	}

	jks, err := readJKS(initEnv, cfg)
	if err != nil {
		common.Throw(rt, err)
	}
	return rt.ToValue(jks)
}

// readJKS loads the keystore via the init-environment filesystem and converts
// its entries to PEM. Only the JKS format is supported; PKCS#12 fails to decode.
func readJKS(initEnv *common.InitEnvironment, cfg JKSConfig) (*JKS, error) {
	if cfg.Path == "" {
		return nil, errors.New("JKS path is required")
	}

	fs, ok := initEnv.FileSystems["file"]
	if !ok {
		return nil, errors.New("no file system available to load the keystore")
	}
	f, err := fs.Open(initEnv.GetAbsFilePath(cfg.Path))
	if err != nil {
		return nil, fmt.Errorf("opening keystore %s: %w", cfg.Path, err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("reading keystore %s: %w", cfg.Path, err)
	}

	return jksToPEM(data, cfg)
}

// jksToPEM decodes JKS keystore bytes and converts the configured entries to
// PEM. Only the JKS format is supported; PKCS#12 fails to decode.
func jksToPEM(data []byte, cfg JKSConfig) (*JKS, error) {
	ks := keystore.New()
	if err := ks.Load(bytes.NewReader(data), []byte(cfg.Password)); err != nil {
		return nil, fmt.Errorf("decoding JKS keystore (only the JKS format is supported, not PKCS#12): %w", err)
	}

	jks := &JKS{}

	// Client key and certificate chain (keystore). Optional: a truststore-only
	// JKS has no private key entry.
	if cfg.ClientKeyAlias != "" {
		pke, err := ks.GetPrivateKeyEntry(cfg.ClientKeyAlias, []byte(cfg.ClientKeyPassword))
		if err != nil {
			return nil, fmt.Errorf("reading client key entry %q: %w", cfg.ClientKeyAlias, err)
		}
		jks.ClientKeyPem = encodePEM("PRIVATE KEY", pke.PrivateKey)
		for _, c := range pke.CertificateChain {
			jks.ClientCertsPem = append(jks.ClientCertsPem, encodePEM("CERTIFICATE", c.Content))
		}
	}

	// Server CA (truststore). Optional: a keystore-only JKS has no CA entry.
	if cfg.ServerCaAlias != "" {
		tce, err := ks.GetTrustedCertificateEntry(cfg.ServerCaAlias)
		if err != nil {
			return nil, fmt.Errorf("reading server CA entry %q: %w", cfg.ServerCaAlias, err)
		}
		jks.ServerCaPem = encodePEM("CERTIFICATE", tce.Certificate.Content)
	}

	return jks, nil
}

func encodePEM(blockType string, der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}))
}
