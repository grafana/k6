package netext

import (
	"context"
	"crypto/tls"

	"github.com/quic-go/quic-go"
)

// QUICDial uses the k6 Dialer's DNS resolution, IP blacklisting, and host
// overrides to resolve the target address, then establishes a QUIC connection.
func (d *Dialer) QUICDial(
	ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config,
) (*quic.Conn, error) {
	dialAddr, err := d.getDialAddr(addr)
	if err != nil {
		return nil, err
	}
	return quic.DialAddrEarly(ctx, dialAddr.String(), tlsCfg, cfg)
}
