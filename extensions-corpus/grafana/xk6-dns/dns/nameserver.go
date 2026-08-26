package dns

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// Nameserver represents a DNS nameserver.
type Nameserver struct {
	// IPAddr is the IP address of the nameserver.
	IP net.IP

	// Port is the port of the nameserver.
	Port uint16
}

// Addr returns the address of the nameserver as a string.
// IPv6 addresses are wrapped in brackets to form valid host:port notation.
func (n Nameserver) Addr() string {
	// Check if it's an IPv6 address (To4() returns nil for IPv6)
	if n.IP.To4() == nil {
		return "[" + n.IP.String() + "]:" + strconv.Itoa(int(n.Port))
	}
	return n.IP.String() + ":" + strconv.Itoa(int(n.Port))
}

// parseNameserverAddr parses a nameserver address string into an IP and a port.
//
// It expects the `addr` to be in the format `ip` or `ip[:port]`. Where `ip` can be an IPv4 or an IPv6 address.
func parseNameserverAddr(addr string) (Nameserver, error) {
	hostStr, port, err := parseHostAndPort(addr)
	if err != nil {
		return Nameserver{}, err
	}

	// Parse the host part into an IP
	ip := net.ParseIP(hostStr)
	if ip == nil {
		return Nameserver{}, fmt.Errorf("invalid nameserver IP address: %s", hostStr)
	}

	return Nameserver{ip, port}, nil
}

func parseHostAndPort(addr string) (string, uint16, error) {
	var err error
	var host string
	var defaultDNSPort uint16 = 53

	// IPv6 addresses can contain colons, so we need to check if the error is due to an IPv6 address
	// without a port, or if it's an actual error.
	if strings.Contains(addr, "]") {
		// Try to trim the brackets from the IPv6 address and parse it without the port
		if addr[0] == '[' && addr[len(addr)-1] == ']' {
			host = addr[1 : len(addr)-1]
			return host, defaultDNSPort, nil
		}
	}

	if !strings.Contains(addr, ":") {
		return addr, defaultDNSPort, nil
	}

	// Check if it's a bare IPv6 address (without brackets, without port)
	// IPv6 addresses contain colons but aren't in bracket notation.
	// This also handles IPv4-mapped IPv6 addresses like ::ffff:192.0.2.1
	if ip := net.ParseIP(addr); ip != nil {
		// It's a valid IP address without port - use default port
		return addr, defaultDNSPort, nil
	}

	// Check if the address contains a port
	// Split the host and the port from the address string
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid nameserver address format: %w", err)
	}

	// SplitHostPort doesn't do any sort of validation on the host and port, and will
	// accept any valid string as a port. Thus we verify its a number ourselves.
	portInt, portErr := strconv.Atoi(port)
	if portErr != nil {
		return "", 0, fmt.Errorf("nameserver port not a number %s: %w", port, portErr)
	}

	// Check if the port is within the valid port range
	if portInt < 0 || portInt > 65535 {
		return "", 0, fmt.Errorf("nameserver port out of range %d", portInt)
	}

	return host, uint16(portInt), nil
}
