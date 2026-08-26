package icmp

import (
	"crypto/rand"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/v2/js/modules"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const maxUint16 = 1<<16 - 1

type pinger struct {
	opts *pingerOptions

	vu      modules.VU
	log     logrus.FieldLogger
	metrics *icmpMetrics

	targetIP      *net.IPAddr
	targetAddr    net.Addr
	ip6           bool
	ipver         string
	packetConn    *icmp.PacketConn
	hopLimitSet   bool
	data          []byte
	sentAt        []time.Time
	seqs          []int
	received      []bool
	sentCount     int
	receivedCount int

	callChan chan func() error
	stop     chan struct{}
	mu       sync.RWMutex
}

func newPinger(opts *pingerOptions, vu modules.VU, log logrus.FieldLogger, metrics *icmpMetrics) *pinger {
	return &pinger{
		opts:     opts,
		vu:       vu,
		log:      log,
		metrics:  metrics,
		sentAt:   make([]time.Time, opts.count),
		seqs:     make([]int, opts.count),
		received: make([]bool, opts.count),
		stop:     make(chan struct{}),
		callChan: make(chan func() error),
	}
}

func (r *pinger) run() (bool, error) {
	err := r.resolve()
	if err != nil {
		return false, err
	}

	err = r.setup()
	if err != nil {
		return false, err
	}

	r.loop()

	success := 100.0*float64(r.receivedCount)/float64(r.sentCount) >= r.opts.threshold

	return success, nil
}

func (r *pinger) resolve() error {
	startedAt := time.Now()

	defer func() { r.addDurationMetrics(startedAt, r.metrics.icmpResolve) }()

	ip, _, err := r.vu.State().GetAddrResolver().ResolveAddr(r.opts.target)
	if err != nil {
		r.addErrorMetrics()

		return err
	}

	r.targetIP = &net.IPAddr{IP: ip}
	r.ip6 = ip.To4() == nil

	if r.ip6 {
		r.ipver = "ip6"
	} else {
		r.ipver = "ip4"
	}

	return nil
}

func (r *pinger) setup() error {
	startedAt := time.Now()

	defer func() { r.addDurationMetrics(startedAt, r.metrics.icmpSetup) }()

	src := r.opts.source
	if len(src) == 0 {
		src = "0.0.0.0"
		if r.ip6 {
			src = "::"
		}
	}

	var (
		addr net.Addr
		err  error
	)

	conn := r.setupUnpriv(src)
	if conn == nil {
		conn, err = r.setupPriv(src)
		if err != nil {
			return err
		}

		addr = r.targetIP
	} else {
		addr = &net.UDPAddr{IP: r.targetIP.IP, Zone: r.targetIP.Zone}
	}

	r.packetConn = conn
	r.targetAddr = addr

	if r.ip6 {
		err = r.packetConn.IPv6PacketConn().SetControlMessage(ipv6.FlagHopLimit, true)
	} else {
		err = r.packetConn.IPv4PacketConn().SetControlMessage(ipv4.FlagTTL, true)
	}

	if err != nil {
		r.log.WithError(err).Debug("Failed to set Control Message for retrieving TTL/HopLimit")
	}

	r.hopLimitSet = err == nil

	r.data = make([]byte, r.opts.size)
	if _, err = rand.Read(r.data); err != nil {
		return err
	}

	return nil
}

func (r *pinger) setupUnpriv(src string) *icmp.PacketConn {
	unpriv := runtime.GOOS == "darwin" || runtime.GOOS == "linux"
	if !unpriv {
		return nil
	}

	proto := "udp4"
	if r.ip6 {
		proto = "udp6"
	}

	conn, err := icmp.ListenPacket(proto, src)
	if err != nil {
		r.log.WithError(err).Warn("Failed to listen for ICMP packets without elevated privileges")

		return nil
	}

	r.log.Debug("Listening for ICMP packets without elevated privileges")

	return conn
}

func (r *pinger) setupPriv(src string) (*icmp.PacketConn, error) {
	proto := "ip4:icmp"
	if r.ip6 {
		proto = "ip6:ipv6-icmp"
	}

	conn, err := icmp.ListenPacket(proto, src)
	if err != nil {
		r.log.WithError(err).Error("Failed to listen for ICMP packets with elevated privileges")

		return nil, err
	}

	r.log.Debug("Listening for ICMP packets with elevated privileges")

	return conn, nil
}
