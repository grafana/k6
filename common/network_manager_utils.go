package common

import (
	"fmt"
	"net"
	"time"

	k6lib "go.k6.io/k6/lib"
	k6netext "go.k6.io/k6/lib/netext"
	k6types "go.k6.io/k6/lib/types"
)

func checkBlockedHosts(host string, blockedHosts *k6types.HostnameTrie) error {
	if blockedHosts == nil {
		return nil
	}
	if match, blocked := blockedHosts.Contains(host); blocked {
		return fmt.Errorf("hostname %s is in a blocked pattern (%s)", host, match)
	}
	return nil
}

func checkBlockedIPs(ip net.IP, blockedIPs []*k6lib.IPNet) error {
	for _, ipnet := range blockedIPs {
		if ipnet.Contains(ip) {
			// TODO: Return netext.BlackListedIPError here once its private
			// fields are exported, or there's a constructor for it.
			return fmt.Errorf("IP (%s) is in a blacklisted range (%s)", ip, ipnet)
		}
	}
	return nil
}

// Returns a new Resolver.
// Copied with minor changes from
// https://github.com/grafana/k6/blob/fb70bc6f3d3f22a40e65f32deea3cea1b6d70a76/js/runner.go#L459
func newResolver(conf k6types.DNSConfig) (k6netext.Resolver, error) {
	ttl, err := parseTTL(conf.TTL.String)
	if err != nil {
		return nil, err
	}

	dnsSel := conf.Select
	if !dnsSel.Valid {
		dnsSel = k6types.DefaultDNSConfig().Select
	}
	dnsPol := conf.Policy
	if !dnsPol.Valid {
		dnsPol = k6types.DefaultDNSConfig().Policy
	}
	return k6netext.NewResolver(
		net.LookupIP, ttl, dnsSel.DNSSelect, dnsPol.DNSPolicy), nil
}

// Parse a string representation of TTL to time.Duration.
// Copied from https://github.com/grafana/k6/blob/fb70bc6f3d3f22a40e65f32deea3cea1b6d70a76/js/runner.go#L479
func parseTTL(ttlS string) (time.Duration, error) {
	ttl := time.Duration(0)
	switch ttlS {
	case "inf":
		// cache "infinitely"
		ttl = time.Hour * 24 * 365
	case "0":
		// disable cache
	case "":
		ttlS = k6types.DefaultDNSConfig().TTL.String
		fallthrough
	default:
		var err error
		ttl, err = k6types.ParseExtendedDuration(ttlS)
		if ttl < 0 || err != nil {
			return ttl, fmt.Errorf("invalid DNS TTL: %s", ttlS)
		}
	}
	return ttl, nil
}
