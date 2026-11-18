package tests

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clientCertCA issues the certificates used by the client certificate tests.
type clientCertCA struct {
	cert *x509.Certificate
	key  crypto.Signer
	pool *x509.CertPool
}

// clientCertPEM is a PEM-encoded client certificate and its key.
type clientCertPEM struct {
	Cert string
	Key  string
}

type clientCertKeyType int

const (
	clientCertKeyRSA clientCertKeyType = iota
	clientCertKeyECDSA
)

func newClientCertCA(t *testing.T) *clientCertCA {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          clientCertSerial(t),
		Subject:               pkix.Name{CommonName: "k6 client certificate test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(cert)

	return &clientCertCA{cert: cert, key: key, pool: pool}
}

// issueClient returns a client authentication certificate for cn.
func (ca *clientCertCA) issueClient(t *testing.T, cn string, keyType clientCertKeyType) clientCertPEM {
	t.Helper()

	key := clientCertSigner(t, keyType)
	tmpl := &x509.Certificate{
		SerialNumber: clientCertSerial(t),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, key.Public(), ca.key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	return clientCertPEM{
		Cert: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		Key:  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})),
	}
}

// serverCertificate returns a server certificate for the loopback hosts and clientcert.test,
// issued by ca, or self-signed when selfSigned is set.
func (ca *clientCertCA) serverCertificate(t *testing.T, selfSigned bool) tls.Certificate {
	t.Helper()

	key := clientCertSigner(t, clientCertKeyECDSA)
	tmpl := &x509.Certificate{
		SerialNumber: clientCertSerial(t),
		Subject:      pkix.Name{CommonName: "k6 client certificate test server"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost", "clientcert.test"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}
	parent, signer := ca.cert, ca.key
	if selfSigned {
		parent, signer = tmpl, key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, key.Public(), signer)
	require.NoError(t, err)

	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func clientCertSigner(t *testing.T, keyType clientCertKeyType) crypto.Signer {
	t.Helper()

	if keyType == clientCertKeyRSA {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		return key
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key
}

func clientCertSerial(t *testing.T) *big.Int {
	t.Helper()

	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 64))
	require.NoError(t, err)
	return n
}

type clientCertServerOptions struct {
	clientAuth tls.ClientAuthType
	h2         bool
	selfSigned bool
	minVersion uint16
	maxVersion uint16
}

// newClientCertServer starts an HTTPS server. Its pages report the client certificate CN
// (or "none") and the HTTP major version as "cn:<cn> proto:<major>"; /ws echoes WebSocket
// messages prefixed with "cn:<cn>:".
func newClientCertServer(t *testing.T, ca *clientCertCA, opts clientCertServerOptions) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if !assert.NoError(t, err) {
			return
		}
		defer conn.Close() //nolint:errcheck
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(mt, []byte("cn:"+clientCertCN(r)+":"+string(msg))); err != nil {
				return
			}
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := fmt.Fprintf(w,
			`<!doctype html><html><head><link rel="icon" href="data:,"></head><body>cn:%s proto:%d</body></html>`,
			clientCertCN(r), r.ProtoMajor)
		assert.NoError(t, err)
	})

	srv := httptest.NewUnstartedServer(mux)
	srv.EnableHTTP2 = opts.h2
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{ca.serverCertificate(t, opts.selfSigned)},
		ClientAuth:   opts.clientAuth,
		ClientCAs:    ca.pool,
		MinVersion:   opts.minVersion,
		MaxVersion:   opts.maxVersion,
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	return srv
}

func clientCertCN(r *http.Request) string {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return "none"
	}
	return r.TLS.PeerCertificates[0].Subject.CommonName
}

// newClientCertConnectProxy starts an HTTP CONNECT proxy that reports each CONNECT target it
// tunnels to connects. With a non-empty username it answers 407 until the request carries
// the matching Basic credentials.
func newClientCertConnectProxy(t *testing.T, username, password string) (*httptest.Server, <-chan string) {
	t.Helper()

	connects := make(chan string, 64)
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "CONNECT only", http.StatusMethodNotAllowed)
			return
		}
		if username != "" && r.Header.Get("Proxy-Authorization") != wantAuth {
			w.Header().Set("Proxy-Authenticate", `Basic realm="k6"`)
			w.WriteHeader(http.StatusProxyAuthRequired)
			return
		}
		connects <- "CONNECT " + r.Host

		target, err := (&net.Dialer{}).DialContext(r.Context(), "tcp", r.Host)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		hj, ok := w.(http.Hijacker)
		if !assert.True(t, ok) {
			_ = target.Close()
			return
		}
		client, buf, err := hj.Hijack()
		if !assert.NoError(t, err) {
			_ = target.Close()
			return
		}
		if _, err := io.WriteString(client, "HTTP/1.1 200 Connection established\r\n\r\n"); err != nil {
			_ = client.Close()
			_ = target.Close()
			return
		}
		go clientCertTunnel(client, buf.Reader, target)
	}))
	t.Cleanup(srv.Close)

	return srv, connects
}

// clientCertTunnel copies between client and target in both directions, propagating
// half-closes, and closes both once both directions finish.
func clientCertTunnel(client net.Conn, clientReader io.Reader, target net.Conn) {
	done := make(chan struct{}, 2)
	half := func(dst net.Conn, src io.Reader) {
		_, _ = io.Copy(dst, src)
		if cw, ok := dst.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
		}
		done <- struct{}{}
	}
	go half(target, clientReader)
	go half(client, target)
	<-done
	<-done
	_ = client.Close()
	_ = target.Close()
}
