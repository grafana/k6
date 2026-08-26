package xk6ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/spf13/afero"
)

func testPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return string(pem.EncodeToMemory(block))
}

func TestRsaKeyAuthMethod_InlinePrivateKey(t *testing.T) {
	t.Parallel()
	// MemMapFs guarantees no key file exists on disk; inline contents must be used.
	k := &K6SSH{fs: afero.NewMemMapFs()}
	auth, err := k.rsaKeyAuthMethod(ConnectionOptions{PrivateKey: testPEM(t)})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if auth == nil {
		t.Fatal("expected an auth method, got nil")
	}
}

func TestRsaKeyAuthMethod_InlineTakesPrecedenceOverPath(t *testing.T) {
	t.Parallel()
	// RsaKey points at a nonexistent path; inline contents should still win.
	k := &K6SSH{fs: afero.NewMemMapFs()}
	auth, err := k.rsaKeyAuthMethod(ConnectionOptions{
		PrivateKey: testPEM(t),
		RsaKey:     "/does/not/exist",
	})
	if err != nil {
		t.Fatalf("expected inline key to take precedence, got %v", err)
	}
	if auth == nil {
		t.Fatal("expected an auth method, got nil")
	}
}

func TestRsaKeyAuthMethod_InvalidInlineKey(t *testing.T) {
	t.Parallel()
	k := &K6SSH{fs: afero.NewMemMapFs()}
	if _, err := k.rsaKeyAuthMethod(ConnectionOptions{PrivateKey: "not a key"}); err == nil {
		t.Fatal("expected an error for invalid key contents")
	}
}
