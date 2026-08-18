package testscript

import (
	"context"
	"crypto/tls"
	"net"
)

func dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, network, address)
}

func lookupHost(ctx context.Context, host string) ([]string, error) {
	return net.DefaultResolver.LookupHost(ctx, host)
}

func tlsClient(ctx context.Context, conn net.Conn, config *tls.Config) (net.Conn, error) {
	if config == nil {
		config = &tls.Config{}
	}
	tlsConn := tls.Client(conn, config)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return tlsConn, nil
}
