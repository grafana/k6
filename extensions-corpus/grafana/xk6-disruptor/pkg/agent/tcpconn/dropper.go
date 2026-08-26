package tcpconn

import (
	"fmt"
	"hash/crc32"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// Dropper is an interface implemented by objects that decide whether a packet should be dropped (rejected).
type Dropper interface {
	Drop(packetBytes []byte) bool
}

// TCPConnectionDropper is a dropper that drops a defined percentage of TCP the connections it sees.
type TCPConnectionDropper struct {
	DropRate float64
}

// Drop decides whether a packet should be dropped by taking the modulus of hash of the connection it belongs to and
// comparing it to a threshold derived from DropRate.
func (tcd TCPConnectionDropper) Drop(packetBytes []byte) bool {
	packet := gopacket.NewPacket(packetBytes, layers.LayerTypeIPv4, gopacket.Default)

	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		return false
	}
	ip, _ := ipLayer.(*layers.IPv4)

	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer == nil {
		return false
	}
	tcp, _ := tcpLayer.(*layers.TCP)

	// fourTuple uniquely identifies this connection by its 4-tuple: source address and port, and destination address
	// and port.
	fourTuple := fmt.Sprintf("%v:%d:%v:%d", ip.SrcIP, tcp.SrcPort, ip.DstIP, tcp.DstPort)

	hash := crc32.NewIEEE()
	_, _ = hash.Write([]byte(fourTuple))
	checksum := hash.Sum32()

	return (checksum % 100) < uint32(100*tcd.DropRate)
}
