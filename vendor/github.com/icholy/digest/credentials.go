package digest

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/icholy/digest/internal/param"
)

// Credentials is a parsed version of the Authorization header
type Credentials struct {
	Username  string
	Realm     string
	Nonce     string
	URI       string
	Response  string
	Algorithm string
	Cnonce    string
	Opaque    string
	QOP       string
	Nc        int
	Userhash  bool
}

// ParseCredentials parses the Authorization header value into credentials
func ParseCredentials(s string) (*Credentials, error) {
	s, ok := strings.CutPrefix(s, Prefix)
	if !ok {
		return nil, errors.New("digest: invalid credentials prefix")
	}
	pp, err := param.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("digest: invalid credentials: %w", err)
	}
	var c Credentials
	for _, p := range pp {
		switch p.Key {
		case "username":
			c.Username = p.Value
		case "realm":
			c.Realm = p.Value
		case "nonce":
			c.Nonce = p.Value
		case "uri":
			c.URI = p.Value
		case "response":
			c.Response = p.Value
		case "algorithm":
			c.Algorithm = p.Value
		case "cnonce":
			c.Cnonce = p.Value
		case "opaque":
			c.Opaque = p.Value
		case "qop":
			c.QOP = p.Value
		case "nc":
			nc, err := strconv.ParseInt(p.Value, 16, 32)
			if err != nil {
				return nil, fmt.Errorf("digest: invalid nc: %w", err)
			}
			c.Nc = int(nc)
		case "userhash":
			c.Userhash = strings.ToLower(p.Value) == "true"
		}
	}
	return &c, nil
}

// String formats the credentials into the header format
func (c *Credentials) String() string {
	var pp []param.Param
	pp = append(pp,
		param.Param{
			Key:   "username",
			Value: c.Username,
			Quote: true,
		},
		param.Param{
			Key:   "realm",
			Value: c.Realm,
			Quote: true,
		},
		param.Param{
			Key:   "nonce",
			Value: c.Nonce,
			Quote: true,
		},
		param.Param{
			Key:   "uri",
			Value: c.URI,
			Quote: true,
		},
	)
	if c.Algorithm != "" {
		pp = append(pp, param.Param{
			Key:   "algorithm",
			Value: c.Algorithm,
		})
	}
	if c.QOP != "" {
		pp = append(pp, param.Param{
			Key:   "cnonce",
			Value: c.Cnonce,
			Quote: true,
		})
	}
	if c.Opaque != "" {
		pp = append(pp, param.Param{
			Key:   "opaque",
			Value: c.Opaque,
			Quote: true,
		})
	}
	if c.QOP != "" {
		pp = append(pp,
			param.Param{
				Key:   "qop",
				Value: c.QOP,
			},
			param.Param{
				Key:   "nc",
				Value: fmt.Sprintf("%08x", c.Nc),
			},
		)
	}
	if c.Userhash {
		pp = append(pp, param.Param{
			Key:   "userhash",
			Value: "true",
		})
	}
	// The RFC does not specify an order, but some implementations expect the response to be at the end.
	// See: https://github.com/icholy/digest/issues/8
	pp = append(pp, param.Param{
		Key:   "response",
		Value: c.Response,
		Quote: true,
	})
	return Prefix + param.Format(pp...)
}
