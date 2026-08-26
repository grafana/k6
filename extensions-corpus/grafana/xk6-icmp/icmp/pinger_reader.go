package icmp

import (
	"bytes"
	"fmt"
	"net"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	protoICMPv4 = 1
	protoICMPv6 = 58
)

func (r *pinger) reader() {
	buff := make([]byte, maxUint16)

	for range r.opts.count {
		if r.ip6 {
			r.process(r.read(buff, protoICMPv6, ipv6.ICMPTypeEchoReply, r.reader6))
		} else {
			r.process(r.read(buff, protoICMPv4, ipv4.ICMPTypeEchoReply, r.reader4))
		}
	}

	r.stop <- struct{}{}
}

func (r *pinger) process(pd *pingerDetail, ok bool, err error) {
	if !ok {
		return
	}

	if err != nil {
		r.addErrorMetrics()

		r.callChan <- func() error {
			return r.invokeCallback(pd, err)
		}

		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	idx := pd.seq - r.opts.seq

	if idx < 0 || idx >= r.sentCount {
		r.log.WithField("starting_seq", r.opts.seq).WithField("reply_seq", pd.seq).WithField("sent_count", r.sentCount).
			Warn("Received ICMP reply with no matching sent packet")

		return
	}

	if r.received[idx] {
		r.log.WithField("seq", pd.seq).Warn("Duplicate ICMP reply received")

		return
	}

	r.received[idx] = true
	r.receivedCount++

	pd.sentAt = r.sentAt[idx]
	pd.alive = true

	r.addReceivedMetrics(pd.size, pd.ttl, pd.sentAt)

	r.callChan <- func() error {
		return r.invokeCallback(pd, err)
	}
}

func (r *pinger) reader4(buff []byte) (int, int, net.Addr, error) {
	n, cm, peer, err := r.packetConn.IPv4PacketConn().ReadFrom(buff)
	if err != nil {
		return -1, -1, nil, err
	}

	ttl := -1

	if cm != nil && r.hopLimitSet {
		ttl = cm.TTL
	}

	return n, ttl, peer, nil
}

func (r *pinger) reader6(buff []byte) (int, int, net.Addr, error) {
	n, cm, peer, err := r.packetConn.IPv6PacketConn().ReadFrom(buff)
	if err != nil {
		return -1, -1, nil, err
	}

	hopLimit := -1

	if cm != nil && r.hopLimitSet {
		hopLimit = cm.HopLimit
	}

	return n, hopLimit, peer, nil
}

type readerFunc func([]byte) (int, int, net.Addr, error)

func (r *pinger) read(buff []byte, icmpProto int, icmpType icmp.Type, reader readerFunc) (*pingerDetail, bool, error) {
	pd := newPingerDetail(r.opts, r.opts.target, r.targetIP.String(), r.ipver)

	if err := r.packetConn.SetReadDeadline(time.Now().Add(r.opts.timeout)); err != nil {
		return pd, true, err
	}

	n, ttl, peer, err := reader(buff)
	if err != nil {
		return pd, true, err
	}

	pd.receivedAt = time.Now()
	pd.size = n

	if ttl >= 0 && r.hopLimitSet {
		pd.ttl = ttl
	}

	if peer.String() != r.targetAddr.String() {
		r.log.WithError(errUnexpectedSource).Warnf("got %s, want %s", peer.String(), r.targetAddr.String())

		return nil, false, nil
	}

	msg, err := icmp.ParseMessage(icmpProto, buff[:n])
	if err != nil {
		r.log.WithError(err).Warn("Failed to parse ICMP message")

		return nil, false, nil
	}

	if msg.Type != icmpType {
		return pd, true, fmt.Errorf("%w: got %s, want %s", errUnexpectedMessage, msg.Type, icmpType)
	}

	echo, ok := msg.Body.(*icmp.Echo)
	if !ok {
		return pd, true, fmt.Errorf("%w: unknown body", errUnexpectedMessage)
	}

	if !bytes.Equal(echo.Data, r.data) {
		return pd, true, fmt.Errorf("%w: data mismatch", errUnexpectedMessage)
	}

	pd.id = echo.ID
	pd.seq = echo.Seq

	return pd, true, nil
}
