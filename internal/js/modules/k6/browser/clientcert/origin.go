package clientcert

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// NormalizeOrigin returns the canonical form of an https origin: lowercase host, default port
// removed, path/query/fragment dropped. It rejects other schemes and unparsable input.
func NormalizeOrigin(origin string) (string, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" {
		return "", fmt.Errorf("invalid origin %q", origin)
	}
	if u.Scheme != "https" {
		return "", errors.New("origin must use https")
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", fmt.Errorf("invalid origin %q", origin)
	}
	if port := u.Port(); port != "" && port != "443" {
		return "https://" + net.JoinHostPort(host, port), nil
	}
	if strings.Contains(host, ":") {
		return "https://[" + host + "]", nil
	}
	return "https://" + host, nil
}
