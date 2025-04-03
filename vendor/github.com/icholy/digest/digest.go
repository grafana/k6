package digest

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strings"
)

// Prefix for digest authentication headers
const Prefix = "Digest "

// IsDigest returns true if the header value is a digest auth header
func IsDigest(header string) bool {
	return strings.HasPrefix(header, Prefix)
}

// Options for creating a credentials
type Options struct {
	Method   string
	URI      string
	GetBody  func() (io.ReadCloser, error)
	Count    int
	Username string
	Password string

	// The following are provided for advanced use cases where the client needs
	// to override the default digest calculation behavior. Most users should
	// leave these fields unset.
	A1     string
	Cnonce string
}

// CanDigest checks if the algorithm and qop are supported
func CanDigest(c *Challenge) bool {
	switch strings.ToUpper(c.Algorithm) {
	case "", "MD5", "SHA-256", "SHA-512", "SHA-512-256":
	default:
		return false
	}
	return len(c.QOP) == 0 || c.SupportsQOP("auth") || c.SupportsQOP("auth-int")
}

// Digest creates credentials from a challenge and request options.
// Note: if you want to re-use a challenge, you must increment the Count.
func Digest(chal *Challenge, o Options) (*Credentials, error) {
	cred := &Credentials{
		Username:  o.Username,
		URI:       o.URI,
		Cnonce:    o.Cnonce,
		Nc:        o.Count,
		Realm:     chal.Realm,
		Nonce:     chal.Nonce,
		Algorithm: chal.Algorithm,
		Opaque:    chal.Opaque,
		Userhash:  chal.Userhash,
	}
	// we re-use the same hash.Hash
	var h hash.Hash
	switch strings.ToUpper(cred.Algorithm) {
	case "", "MD5":
		h = md5.New()
	case "SHA-256":
		h = sha256.New()
	case "SHA-512":
		h = sha512.New()
	case "SHA-512-256":
		h = sha512.New512_256()
	default:
		return nil, fmt.Errorf("digest: unsupported algorithm: %q", cred.Algorithm)
	}
	// hash the username if requested
	if cred.Userhash {
		cred.Username = hashf(h, "%s:%s", o.Username, cred.Realm)
	}
	// generate the a1 hash if one was not provided
	a1 := o.A1
	if a1 == "" {
		a1 = hashf(h, "%s:%s:%s", o.Username, cred.Realm, o.Password)
	}
	// generate the response
	switch {
	case len(chal.QOP) == 0:
		cred.Response = hashf(h, "%s:%s:%s",
			a1,
			cred.Nonce,
			hashf(h, "%s:%s", o.Method, o.URI), // A2
		)
	case chal.SupportsQOP("auth"):
		cred.QOP = "auth"
		if cred.Cnonce == "" {
			cred.Cnonce = cnonce()
		}
		if cred.Nc == 0 {
			cred.Nc = 1
		}
		cred.Response = hashf(h, "%s:%s:%08x:%s:%s:%s",
			a1,
			cred.Nonce,
			cred.Nc,
			cred.Cnonce,
			cred.QOP,
			hashf(h, "%s:%s", o.Method, o.URI), // A2
		)
	case chal.SupportsQOP("auth-int"):
		cred.QOP = "auth-int"
		if cred.Cnonce == "" {
			cred.Cnonce = cnonce()
		}
		if cred.Nc == 0 {
			cred.Nc = 1
		}
		hbody, err := hashbody(h, o.GetBody)
		if err != nil {
			return nil, fmt.Errorf("digest: failed to read body for auth-int: %w", err)
		}
		cred.Response = hashf(h, "%s:%s:%08x:%s:%s:%s",
			a1,
			cred.Nonce,
			cred.Nc,
			cred.Cnonce,
			cred.QOP,
			hashf(h, "%s:%s:%s", o.Method, o.URI, hbody), // A2
		)
	default:
		return nil, fmt.Errorf("digest: unsupported qop: %q", strings.Join(chal.QOP, ","))
	}
	return cred, nil
}

func hashf(h hash.Hash, format string, args ...interface{}) string {
	h.Reset()
	fmt.Fprintf(h, format, args...)
	return hex.EncodeToString(h.Sum(nil))
}

func hashbody(h hash.Hash, getbody func() (io.ReadCloser, error)) (string, error) {
	h.Reset()
	if getbody != nil {
		r, err := getbody()
		if err != nil {
			return "", err
		}
		defer r.Close()
		if r != http.NoBody {
			if _, err := io.Copy(h, r); err != nil {
				return "", err
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func cnonce() string {
	b := make([]byte, 8)
	io.ReadFull(rand.Reader, b)
	return hex.EncodeToString(b)
}
