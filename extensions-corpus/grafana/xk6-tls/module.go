// Package tls give access to information access and operations for tls connections
package tls

import (
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/grafana/sobek"
	"go.k6.io/k6-extension-api"
)

type (
	// RootModule is the global module instance that will create Client
	// instances for each VU.
	RootModule struct{}

	// ModuleInstance represents an instance of the JS module.
	ModuleInstance struct {
		vu extensionapi.VU
	}
)

// Ensure the interfaces are implemented correctly
var (
	_ extensionapi.Instance = &ModuleInstance{}
	_ extensionapi.Module   = &RootModule{}
)

// New returns a pointer to a new RootModule instance
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface and returns
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu extensionapi.VU) extensionapi.Instance {
	return &ModuleInstance{
		vu: vu,
	}
}

// Exports implements the modules.Instance interface and returns
// the exports of the JS module.
func (mi *ModuleInstance) Exports() extensionapi.Exports {
	return extensionapi.Exports{Default: mi}
}

// GetCertificate fetches and exposes the peer certificate's details.
func (mi *ModuleInstance) GetCertificate(target string) *sobek.Promise {
	promises, ok := mi.vu.(extensionapi.Promises)
	if !ok {
		panic("extension API promise capability is unavailable")
	}
	promise, resolver := promises.NewPromise()

	network, ok := mi.vu.(extensionapi.Network)
	if !ok {
		resolver.Reject(fmt.Errorf("getCertificate is not allowed to run from the Init Context"))
		return promise
	}

	addr, err := parseTargetAddr(target)
	if err != nil {
		resolver.Reject(err)
		return promise
	}

	go func() {
		netconn, err := network.DialContext(mi.vu.Context(), "tcp", addr.uri)
		if err != nil {
			resolver.Reject(err)
			return
		}
		defer func() {
			_ = netconn.Close()
		}()

		conn := tls.Client(netconn, &tls.Config{
			//nolint:gosec
			// we need to skip the check otherwise any eventual
			// expired certificate will return an error
			InsecureSkipVerify: true,
		})
		if err := conn.HandshakeContext(mi.vu.Context()); err != nil {
			resolver.Reject(err)
			return
		}
		peerCerts := conn.ConnectionState().PeerCertificates
		if len(peerCerts) < 1 {
			resolver.Reject(fmt.Errorf("no certificate found for %s - the server may not be using TLS or the connection failed", target))
			return
		}
		c := peerCerts[0]
		resolver.Resolve(certificate{
			Subject:     pkixName{CommonName: c.Subject.CommonName},
			Issuer:      pkixName{CommonName: c.Issuer.CommonName},
			Issued:      c.NotBefore.UnixMilli(),
			Expires:     c.NotAfter.UnixMilli(),
			Fingerprint: fingerprint(c.Raw),
		})
	}()
	return promise
}

type certificate struct {
	Subject     pkixName
	Issuer      pkixName
	Issued      int64
	Expires     int64
	Fingerprint string
}

func fingerprint(cert []byte) string {
	sum := sha256.Sum256(cert)
	return fmt.Sprintf("%x", sum)
}

type pkixName struct {
	CommonName string
}
type addr struct {
	host, port string
	uri        string
}

func parseTargetAddr(target string) (addr, error) {
	if target == "" {
		return addr{}, fmt.Errorf("target address was not provided")
	}
	port := "443" // default https port

	if !strings.Contains(target, ":") {
		return addr{
			host: target,
			port: port,
			uri:  net.JoinHostPort(target, port),
		}, nil
	}

	h, p, err := net.SplitHostPort(target)
	if err != nil {
		return addr{}, err
	}
	if h == "" {
		return addr{}, fmt.Errorf("the provided target does not contain a valid address in the host:[port] format")
	}

	if p != "" {
		_, parseErr := strconv.ParseUint(p, 10, 16)
		if parseErr != nil {
			return addr{}, fmt.Errorf("the provided target does not contain a valid port %q", p)
		}
		port = p
	}

	return addr{
		host: h,
		port: port,
		uri:  net.JoinHostPort(h, port),
	}, nil
}
