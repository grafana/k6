package types

import (
	"net"
	"strconv"
)

// Host stores information about IP and port
// for a host.
type Host net.TCPAddr

// NewHost creates a pointer to a new address with an IP object.
func NewHost(ip net.IP, portString string) (*Host, error) {
	if len(ip) != net.IPv4len && len(ip) != net.IPv6len {
		return nil, &net.AddrError{Err: "invalid IP address", Addr: ip.String()}
	}

	var port int
	if portString != "" {
		var err error
		if port, err = strconv.Atoi(portString); err != nil {
			return nil, err
		}
	}

	return &Host{
		IP:   ip,
		Port: port,
	}, nil
}

// String converts a Host into a string.
func (h *Host) String() string {
	return (*net.TCPAddr)(h).String()
}

// MarshalText implements the encoding.TextMarshaler interface.
// The encoding is the same as returned by String, with one exception:
// When len(ip) is zero, it returns an empty slice.
func (h *Host) MarshalText() ([]byte, error) {
	if h == nil || len(h.IP) == 0 {
		return []byte(""), nil
	}

	if len(h.IP) != net.IPv4len && len(h.IP) != net.IPv6len {
		return nil, &net.AddrError{Err: "invalid IP address", Addr: h.IP.String()}
	}

	return []byte(h.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// The IP address is expected in a form accepted by ParseIP.
func (h *Host) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return &net.ParseError{Type: "IP address", Text: "<nil>"}
	}

	ip, port, err := splitHostPort(text)
	if err != nil {
		return err
	}

	nh, err := NewHost(ip, port)
	if err != nil {
		return err
	}

	*h = *nh

	return nil
}

func splitHostPort(text []byte) (net.IP, string, error) {
	host, port, err := net.SplitHostPort(string(text))
	if err != nil {
		// This error means that there is no port.
		// Make host the full text.
		host = string(text)
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return nil, "", &net.ParseError{Type: "IP address", Text: host}
	}

	return ip, port, nil
}
