package digest

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/icholy/digest/internal/param"
)

// Challenge is a challenge sent in the WWW-Authenticate header
type Challenge struct {
	Realm     string
	Domain    []string
	Nonce     string
	Opaque    string
	Stale     bool
	Algorithm string
	QOP       []string
	Charset   string
	Userhash  bool
}

// SupportsQOP returns true if the challenge advertises support
// for the provided qop value
func (c *Challenge) SupportsQOP(qop string) bool {
	return slices.Contains(c.QOP, qop)
}

// ParseChallenge parses the WWW-Authenticate header challenge
func ParseChallenge(s string) (*Challenge, error) {
	s, ok := CutPrefix(s)
	if !ok {
		return nil, errors.New("digest: invalid challenge prefix")
	}
	pp, err := param.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("digest: invalid challenge: %w", err)
	}
	var c Challenge
	for _, p := range pp {
		switch p.Key {
		case "realm":
			c.Realm = p.Value
		case "domain":
			c.Domain = strings.Fields(p.Value)
		case "nonce":
			c.Nonce = p.Value
		case "algorithm":
			c.Algorithm = p.Value
		case "stale":
			c.Stale = strings.ToLower(p.Value) == "true"
		case "opaque":
			c.Opaque = p.Value
		case "qop":
			c.QOP = strings.Split(p.Value, ",")
		case "charset":
			c.Charset = p.Value
		case "userhash":
			c.Userhash = strings.ToLower(p.Value) == "true"
		}
	}
	return &c, nil
}

// String returns the foramtted header value
func (c *Challenge) String() string {
	var pp []param.Param
	pp = append(pp, param.Param{
		Key:   "realm",
		Value: c.Realm,
		Quote: true,
	})
	if len(c.Domain) != 0 {
		pp = append(pp, param.Param{
			Key:   "domain",
			Value: strings.Join(c.Domain, " "),
			Quote: true,
		})
	}
	pp = append(pp, param.Param{
		Key:   "nonce",
		Value: c.Nonce,
		Quote: true,
	})
	if c.Opaque != "" {
		pp = append(pp, param.Param{
			Key:   "opaque",
			Value: c.Opaque,
			Quote: true,
		})
	}
	if c.Stale {
		pp = append(pp, param.Param{
			Key:   "stale",
			Value: "true",
		})
	}
	if c.Algorithm != "" {
		pp = append(pp, param.Param{
			Key:   "algorithm",
			Value: c.Algorithm,
		})
	}
	if len(c.QOP) != 0 {
		pp = append(pp, param.Param{
			Key:   "qop",
			Value: strings.Join(c.QOP, ","),
			Quote: true,
		})
	}
	if c.Charset != "" {
		pp = append(pp, param.Param{
			Key:   "charset",
			Value: c.Charset,
		})
	}
	if c.Userhash {
		pp = append(pp, param.Param{
			Key:   "userhash",
			Value: "true",
		})
	}
	return Prefix + param.Format(pp...)
}

// ErrNoChallenge indicates that no WWW-Authenticate headers were found.
var ErrNoChallenge = errors.New("digest: no challenge found")

// FindChallenge returns the first supported challenge in the headers
func FindChallenge(h http.Header) (*Challenge, error) {
	var last error
	for _, header := range h.Values("WWW-Authenticate") {
		if !IsDigest(header) {
			continue
		}
		chal, err := ParseChallenge(header)
		if err == nil && CanDigest(chal) {
			return chal, nil
		}
		if err != nil {
			last = err
		}
	}
	if last != nil {
		return nil, last
	}
	return nil, ErrNoChallenge
}
