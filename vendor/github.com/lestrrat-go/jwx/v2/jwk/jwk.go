//go:generate ../tools/cmd/genjwk.sh

// Package jwk implements JWK as described in https://tools.ietf.org/html/rfc7517
package jwk

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/lestrrat-go/jwx/v2/internal/base64"
	"github.com/lestrrat-go/jwx/v2/internal/ecutil"
	"github.com/lestrrat-go/jwx/v2/internal/json"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/x25519"
)

var registry = json.NewRegistry()

func bigIntToBytes(n *big.Int) ([]byte, error) {
	if n == nil {
		return nil, fmt.Errorf(`invalid *big.Int value`)
	}
	return n.Bytes(), nil
}

// FromRaw creates a jwk.Key from the given key (RSA/ECDSA/symmetric keys).
//
// The constructor auto-detects the type of key to be instantiated
// based on the input type:
//
//   - "crypto/rsa".PrivateKey and "crypto/rsa".PublicKey creates an RSA based key
//   - "crypto/ecdsa".PrivateKey and "crypto/ecdsa".PublicKey creates an EC based key
//   - "crypto/ed25519".PrivateKey and "crypto/ed25519".PublicKey creates an OKP based key
//   - []byte creates a symmetric key
func FromRaw(key interface{}) (Key, error) {
	if key == nil {
		return nil, fmt.Errorf(`jwk.FromRaw requires a non-nil key`)
	}

	var ptr interface{}
	switch v := key.(type) {
	case rsa.PrivateKey:
		ptr = &v
	case rsa.PublicKey:
		ptr = &v
	case ecdsa.PrivateKey:
		ptr = &v
	case ecdsa.PublicKey:
		ptr = &v
	default:
		ptr = v
	}

	switch rawKey := ptr.(type) {
	case *rsa.PrivateKey:
		k := newRSAPrivateKey()
		if err := k.FromRaw(rawKey); err != nil {
			return nil, fmt.Errorf(`failed to initialize %T from %T: %w`, k, rawKey, err)
		}
		return k, nil
	case *rsa.PublicKey:
		k := newRSAPublicKey()
		if err := k.FromRaw(rawKey); err != nil {
			return nil, fmt.Errorf(`failed to initialize %T from %T: %w`, k, rawKey, err)
		}
		return k, nil
	case *ecdsa.PrivateKey:
		k := newECDSAPrivateKey()
		if err := k.FromRaw(rawKey); err != nil {
			return nil, fmt.Errorf(`failed to initialize %T from %T: %w`, k, rawKey, err)
		}
		return k, nil
	case *ecdsa.PublicKey:
		k := newECDSAPublicKey()
		if err := k.FromRaw(rawKey); err != nil {
			return nil, fmt.Errorf(`failed to initialize %T from %T: %w`, k, rawKey, err)
		}
		return k, nil
	case ed25519.PrivateKey:
		k := newOKPPrivateKey()
		if err := k.FromRaw(rawKey); err != nil {
			return nil, fmt.Errorf(`failed to initialize %T from %T: %w`, k, rawKey, err)
		}
		return k, nil
	case ed25519.PublicKey:
		k := newOKPPublicKey()
		if err := k.FromRaw(rawKey); err != nil {
			return nil, fmt.Errorf(`failed to initialize %T from %T: %w`, k, rawKey, err)
		}
		return k, nil
	case x25519.PrivateKey:
		k := newOKPPrivateKey()
		if err := k.FromRaw(rawKey); err != nil {
			return nil, fmt.Errorf(`failed to initialize %T from %T: %w`, k, rawKey, err)
		}
		return k, nil
	case x25519.PublicKey:
		k := newOKPPublicKey()
		if err := k.FromRaw(rawKey); err != nil {
			return nil, fmt.Errorf(`failed to initialize %T from %T: %w`, k, rawKey, err)
		}
		return k, nil
	case []byte:
		k := newSymmetricKey()
		if err := k.FromRaw(rawKey); err != nil {
			return nil, fmt.Errorf(`failed to initialize %T from %T: %w`, k, rawKey, err)
		}
		return k, nil
	default:
		return nil, fmt.Errorf(`invalid key type '%T' for jwk.New`, key)
	}
}

// PublicSetOf returns a new jwk.Set consisting of
// public keys of the keys contained in the set.
//
// This is useful when you are generating a set of private keys, and
// you want to generate the corresponding public versions for the
// users to verify with.
//
// Be aware that all fields will be copied onto the new public key. It is the caller's
// responsibility to remove any fields, if necessary.
func PublicSetOf(v Set) (Set, error) {
	newSet := NewSet()

	n := v.Len()
	for i := 0; i < n; i++ {
		k, ok := v.Key(i)
		if !ok {
			return nil, fmt.Errorf(`key not found`)
		}
		pubKey, err := PublicKeyOf(k)
		if err != nil {
			return nil, fmt.Errorf(`failed to get public key of %T: %w`, k, err)
		}
		if err := newSet.AddKey(pubKey); err != nil {
			return nil, fmt.Errorf(`failed to add key to public key set: %w`, err)
		}
	}

	return newSet, nil
}

// PublicKeyOf returns the corresponding public version of the jwk.Key.
// If `v` is a SymmetricKey, then the same value is returned.
// If `v` is already a public key, the key itself is returned.
//
// If `v` is a private key type that has a `PublicKey()` method, be aware
// that all fields will be copied onto the new public key. It is the caller's
// responsibility to remove any fields, if necessary
//
// If `v` is a raw key, the key is first converted to a `jwk.Key`
func PublicKeyOf(v interface{}) (Key, error) {
	// This should catch all jwk.Key instances
	if pk, ok := v.(PublicKeyer); ok {
		return pk.PublicKey()
	}

	jk, err := FromRaw(v)
	if err != nil {
		return nil, fmt.Errorf(`failed to convert key into JWK: %w`, err)
	}

	return jk.PublicKey()
}

// PublicRawKeyOf returns the corresponding public key of the given
// value `v` (e.g. given *rsa.PrivateKey, *rsa.PublicKey is returned)
// If `v` is already a public key, the key itself is returned.
//
// The returned value will always be a pointer to the public key,
// except when a []byte (e.g. symmetric key, ed25519 key) is passed to `v`.
// In this case, the same []byte value is returned.
func PublicRawKeyOf(v interface{}) (interface{}, error) {
	if pk, ok := v.(PublicKeyer); ok {
		pubk, err := pk.PublicKey()
		if err != nil {
			return nil, fmt.Errorf(`failed to obtain public key from %T: %w`, v, err)
		}

		var raw interface{}
		if err := pubk.Raw(&raw); err != nil {
			return nil, fmt.Errorf(`failed to obtain raw key from %T: %w`, pubk, err)
		}
		return raw, nil
	}

	// This may be a silly idea, but if the user gave us a non-pointer value...
	var ptr interface{}
	switch v := v.(type) {
	case rsa.PrivateKey:
		ptr = &v
	case rsa.PublicKey:
		ptr = &v
	case ecdsa.PrivateKey:
		ptr = &v
	case ecdsa.PublicKey:
		ptr = &v
	default:
		ptr = v
	}

	switch x := ptr.(type) {
	case *rsa.PrivateKey:
		return &x.PublicKey, nil
	case *rsa.PublicKey:
		return x, nil
	case *ecdsa.PrivateKey:
		return &x.PublicKey, nil
	case *ecdsa.PublicKey:
		return x, nil
	case ed25519.PrivateKey:
		return x.Public(), nil
	case ed25519.PublicKey:
		return x, nil
	case x25519.PrivateKey:
		return x.Public(), nil
	case x25519.PublicKey:
		return x, nil
	case []byte:
		return x, nil
	default:
		return nil, fmt.Errorf(`invalid key type passed to PublicKeyOf (%T)`, v)
	}
}

const (
	pmPrivateKey    = `PRIVATE KEY`
	pmPublicKey     = `PUBLIC KEY`
	pmECPrivateKey  = `EC PRIVATE KEY`
	pmRSAPublicKey  = `RSA PUBLIC KEY`
	pmRSAPrivateKey = `RSA PRIVATE KEY`
)

// EncodeX509 encodes the key into a byte sequence in ASN.1 DER format
// suitable for to be PEM encoded. The key can be a jwk.Key or a raw key
// instance, but it must be one of the types supported by `x509` package.
//
// This function will try to do the right thing depending on the key type
// (i.e. switch between `x509.MarshalPKCS1PRivateKey` and `x509.MarshalECPrivateKey`),
// but for public keys, it will always use `x509.MarshalPKIXPublicKey`.
// Please manually perform the encoding if you need more fine grained control
//
// The first return value is the name that can be used for `(pem.Block).Type`.
// The second return value is the encoded byte sequence.
func EncodeX509(v interface{}) (string, []byte, error) {
	// we can't import jwk, so just use the interface
	if key, ok := v.(interface{ Raw(interface{}) error }); ok {
		var raw interface{}
		if err := key.Raw(&raw); err != nil {
			return "", nil, fmt.Errorf(`failed to get raw key out of %T: %w`, key, err)
		}

		v = raw
	}

	// Try to convert it into a certificate
	switch v := v.(type) {
	case *rsa.PrivateKey:
		return pmRSAPrivateKey, x509.MarshalPKCS1PrivateKey(v), nil
	case *ecdsa.PrivateKey:
		marshaled, err := x509.MarshalECPrivateKey(v)
		if err != nil {
			return "", nil, err
		}
		return pmECPrivateKey, marshaled, nil
	case ed25519.PrivateKey:
		marshaled, err := x509.MarshalPKCS8PrivateKey(v)
		if err != nil {
			return "", nil, err
		}
		return pmPrivateKey, marshaled, nil
	case *rsa.PublicKey, *ecdsa.PublicKey, ed25519.PublicKey:
		marshaled, err := x509.MarshalPKIXPublicKey(v)
		if err != nil {
			return "", nil, err
		}
		return pmPublicKey, marshaled, nil
	default:
		return "", nil, fmt.Errorf(`unsupported type %T for ASN.1 DER encoding`, v)
	}
}

// EncodePEM encodes the key into a PEM encoded ASN.1 DER format.
// The key can be a jwk.Key or a raw key instance, but it must be one of
// the types supported by `x509` package.
//
// Internally, it uses the same routine as `jwk.EncodeX509()`, and therefore
// the same caveats apply
func EncodePEM(v interface{}) ([]byte, error) {
	typ, marshaled, err := EncodeX509(v)
	if err != nil {
		return nil, fmt.Errorf(`failed to encode key in x509: %w`, err)
	}

	block := &pem.Block{
		Type:  typ,
		Bytes: marshaled,
	}
	return pem.EncodeToMemory(block), nil
}

// DecodePEM decodes a key in PEM encoded ASN.1 DER format.
// and returns a raw key
func DecodePEM(src []byte) (interface{}, []byte, error) {
	block, rest := pem.Decode(src)
	if block == nil {
		return nil, nil, fmt.Errorf(`failed to decode PEM data`)
	}

	switch block.Type {
	// Handle the semi-obvious cases
	case pmRSAPrivateKey:
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf(`failed to parse PKCS1 private key: %w`, err)
		}
		return key, rest, nil
	case pmRSAPublicKey:
		key, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf(`failed to parse PKCS1 public key: %w`, err)
		}
		return key, rest, nil
	case pmECPrivateKey:
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf(`failed to parse EC private key: %w`, err)
		}
		return key, rest, nil
	case pmPublicKey:
		// XXX *could* return dsa.PublicKey
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf(`failed to parse PKIX public key: %w`, err)
		}
		return key, rest, nil
	case pmPrivateKey:
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf(`failed to parse PKCS8 private key: %w`, err)
		}
		return key, rest, nil
	case "CERTIFICATE":
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf(`failed to parse certificate: %w`, err)
		}
		return cert.PublicKey, rest, nil
	default:
		return nil, nil, fmt.Errorf(`invalid PEM block type %s`, block.Type)
	}
}

// ParseRawKey is a combination of ParseKey and Raw. It parses a single JWK key,
// and assigns the "raw" key to the given parameter. The key must either be
// a pointer to an empty interface, or a pointer to the actual raw key type
// such as *rsa.PrivateKey, *ecdsa.PublicKey, *[]byte, etc.
func ParseRawKey(data []byte, rawkey interface{}) error {
	key, err := ParseKey(data)
	if err != nil {
		return fmt.Errorf(`failed to parse key: %w`, err)
	}

	if err := key.Raw(rawkey); err != nil {
		return fmt.Errorf(`failed to assign to raw key variable: %w`, err)
	}

	return nil
}

type setDecodeCtx struct {
	json.DecodeCtx
	ignoreParseError bool
}

func (ctx *setDecodeCtx) IgnoreParseError() bool {
	return ctx.ignoreParseError
}

// ParseKey parses a single key JWK. Unlike `jwk.Parse` this method will
// report failure if you attempt to pass a JWK set. Only use this function
// when you know that the data is a single JWK.
//
// Given a WithPEM(true) option, this function assumes that the given input
// is PEM encoded ASN.1 DER format key.
//
// Note that a successful parsing of any type of key does NOT necessarily
// guarantee a valid key. For example, no checks against expiration dates
// are performed for certificate expiration, no checks against missing
// parameters are performed, etc.
func ParseKey(data []byte, options ...ParseOption) (Key, error) {
	var parsePEM bool
	var localReg *json.Registry
	for _, option := range options {
		//nolint:forcetypeassert
		switch option.Ident() {
		case identPEM{}:
			parsePEM = option.Value().(bool)
		case identLocalRegistry{}:
			// in reality you can only pass either withLocalRegistry or
			// WithTypedField, but since withLocalRegistry is used only by us,
			// we skip checking
			localReg = option.Value().(*json.Registry)
		case identTypedField{}:
			pair := option.Value().(typedFieldPair)
			if localReg == nil {
				localReg = json.NewRegistry()
			}
			localReg.Register(pair.Name, pair.Value)
		case identIgnoreParseError{}:
			return nil, fmt.Errorf(`jwk.WithIgnoreParseError() cannot be used for ParseKey()`)
		}
	}

	if parsePEM {
		raw, _, err := DecodePEM(data)
		if err != nil {
			return nil, fmt.Errorf(`failed to parse PEM encoded key: %w`, err)
		}
		return FromRaw(raw)
	}

	var hint struct {
		Kty string          `json:"kty"`
		D   json.RawMessage `json:"d"`
	}

	if err := json.Unmarshal(data, &hint); err != nil {
		return nil, fmt.Errorf(`failed to unmarshal JSON into key hint: %w`, err)
	}

	var key Key
	switch jwa.KeyType(hint.Kty) {
	case jwa.RSA:
		if len(hint.D) > 0 {
			key = newRSAPrivateKey()
		} else {
			key = newRSAPublicKey()
		}
	case jwa.EC:
		if len(hint.D) > 0 {
			key = newECDSAPrivateKey()
		} else {
			key = newECDSAPublicKey()
		}
	case jwa.OctetSeq:
		key = newSymmetricKey()
	case jwa.OKP:
		if len(hint.D) > 0 {
			key = newOKPPrivateKey()
		} else {
			key = newOKPPublicKey()
		}
	default:
		return nil, fmt.Errorf(`invalid key type from JSON (%s)`, hint.Kty)
	}

	if localReg != nil {
		dcKey, ok := key.(json.DecodeCtxContainer)
		if !ok {
			return nil, fmt.Errorf(`typed field was requested, but the key (%T) does not support DecodeCtx`, key)
		}
		dc := json.NewDecodeCtx(localReg)
		dcKey.SetDecodeCtx(dc)
		defer func() { dcKey.SetDecodeCtx(nil) }()
	}

	if err := json.Unmarshal(data, key); err != nil {
		return nil, fmt.Errorf(`failed to unmarshal JSON into key (%T): %w`, key, err)
	}

	return key, nil
}

// Parse parses JWK from the incoming []byte.
//
// For JWK sets, this is a convenience function. You could just as well
// call `json.Unmarshal` against an empty set created by `jwk.NewSet()`
// to parse a JSON buffer into a `jwk.Set`.
//
// This function exists because many times the user does not know before hand
// if a JWK(s) resource at a remote location contains a single JWK key or
// a JWK set, and `jwk.Parse()` can handle either case, returning a JWK Set
// even if the data only contains a single JWK key
//
// If you are looking for more information on how JWKs are parsed, or if
// you know for sure that you have a single key, please see the documentation
// for `jwk.ParseKey()`.
func Parse(src []byte, options ...ParseOption) (Set, error) {
	var parsePEM bool
	var localReg *json.Registry
	var ignoreParseError bool
	for _, option := range options {
		//nolint:forcetypeassert
		switch option.Ident() {
		case identPEM{}:
			parsePEM = option.Value().(bool)
		case identIgnoreParseError{}:
			ignoreParseError = option.Value().(bool)
		case identTypedField{}:
			pair := option.Value().(typedFieldPair)
			if localReg == nil {
				localReg = json.NewRegistry()
			}
			localReg.Register(pair.Name, pair.Value)
		}
	}

	s := NewSet()

	if parsePEM {
		src = bytes.TrimSpace(src)
		for len(src) > 0 {
			raw, rest, err := DecodePEM(src)
			if err != nil {
				return nil, fmt.Errorf(`failed to parse PEM encoded key: %w`, err)
			}
			key, err := FromRaw(raw)
			if err != nil {
				return nil, fmt.Errorf(`failed to create jwk.Key from %T: %w`, raw, err)
			}
			if err := s.AddKey(key); err != nil {
				return nil, fmt.Errorf(`failed to add jwk.Key to set: %w`, err)
			}
			src = bytes.TrimSpace(rest)
		}
		return s, nil
	}

	if localReg != nil || ignoreParseError {
		dcKs, ok := s.(KeyWithDecodeCtx)
		if !ok {
			return nil, fmt.Errorf(`typed field was requested, but the key set (%T) does not support DecodeCtx`, s)
		}
		dc := &setDecodeCtx{
			DecodeCtx:        json.NewDecodeCtx(localReg),
			ignoreParseError: ignoreParseError,
		}
		dcKs.SetDecodeCtx(dc)
		defer func() { dcKs.SetDecodeCtx(nil) }()
	}

	if err := json.Unmarshal(src, s); err != nil {
		return nil, fmt.Errorf(`failed to unmarshal JWK set: %w`, err)
	}

	return s, nil
}

// ParseReader parses a JWK set from the incoming byte buffer.
func ParseReader(src io.Reader, options ...ParseOption) (Set, error) {
	// meh, there's no way to tell if a stream has "ended" a single
	// JWKs except when we encounter an EOF, so just... ReadAll
	buf, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf(`failed to read from io.Reader: %w`, err)
	}

	return Parse(buf, options...)
}

// ParseString parses a JWK set from the incoming string.
func ParseString(s string, options ...ParseOption) (Set, error) {
	return Parse([]byte(s), options...)
}

// AssignKeyID is a convenience function to automatically assign the "kid"
// section of the key, if it already doesn't have one. It uses Key.Thumbprint
// method with crypto.SHA256 as the default hashing algorithm
func AssignKeyID(key Key, options ...AssignKeyIDOption) error {
	if _, ok := key.Get(KeyIDKey); ok {
		return nil
	}

	hash := crypto.SHA256
	for _, option := range options {
		//nolint:forcetypeassert
		switch option.Ident() {
		case identThumbprintHash{}:
			hash = option.Value().(crypto.Hash)
		}
	}

	h, err := key.Thumbprint(hash)
	if err != nil {
		return fmt.Errorf(`failed to generate thumbprint: %w`, err)
	}

	if err := key.Set(KeyIDKey, base64.EncodeToString(h)); err != nil {
		return fmt.Errorf(`failed to set "kid": %w`, err)
	}

	return nil
}

func cloneKey(src Key) (Key, error) {
	var dst Key
	switch src.(type) {
	case RSAPrivateKey:
		dst = newRSAPrivateKey()
	case RSAPublicKey:
		dst = newRSAPublicKey()
	case ECDSAPrivateKey:
		dst = newECDSAPrivateKey()
	case ECDSAPublicKey:
		dst = newECDSAPublicKey()
	case OKPPrivateKey:
		dst = newOKPPrivateKey()
	case OKPPublicKey:
		dst = newOKPPublicKey()
	case SymmetricKey:
		dst = newSymmetricKey()
	default:
		return nil, fmt.Errorf(`unknown key type %T`, src)
	}

	for _, pair := range src.makePairs() {
		//nolint:forcetypeassert
		key := pair.Key.(string)
		if err := dst.Set(key, pair.Value); err != nil {
			return nil, fmt.Errorf(`failed to set %q: %w`, key, err)
		}
	}
	return dst, nil
}

// Pem serializes the given jwk.Key in PEM encoded ASN.1 DER format,
// using either PKCS8 for private keys and PKIX for public keys.
// If you need to encode using PKCS1 or SEC1, you must do it yourself.
//
// # Argument must be of type jwk.Key or jwk.Set
//
// Currently only EC (including Ed25519) and RSA keys (and jwk.Set
// comprised of these key types) are supported.
func Pem(v interface{}) ([]byte, error) {
	var set Set
	switch v := v.(type) {
	case Key:
		set = NewSet()
		if err := set.AddKey(v); err != nil {
			return nil, fmt.Errorf(`failed to add key to set: %w`, err)
		}
	case Set:
		set = v
	default:
		return nil, fmt.Errorf(`argument to Pem must be either jwk.Key or jwk.Set: %T`, v)
	}

	var ret []byte
	for i := 0; i < set.Len(); i++ {
		key, _ := set.Key(i)
		typ, buf, err := asnEncode(key)
		if err != nil {
			return nil, fmt.Errorf(`failed to encode content for key #%d: %w`, i, err)
		}

		var block pem.Block
		block.Type = typ
		block.Bytes = buf
		ret = append(ret, pem.EncodeToMemory(&block)...)
	}
	return ret, nil
}

func asnEncode(key Key) (string, []byte, error) {
	switch key := key.(type) {
	case RSAPrivateKey, ECDSAPrivateKey, OKPPrivateKey:
		var rawkey interface{}
		if err := key.Raw(&rawkey); err != nil {
			return "", nil, fmt.Errorf(`failed to get raw key from jwk.Key: %w`, err)
		}
		buf, err := x509.MarshalPKCS8PrivateKey(rawkey)
		if err != nil {
			return "", nil, fmt.Errorf(`failed to marshal PKCS8: %w`, err)
		}
		return pmPrivateKey, buf, nil
	case RSAPublicKey, ECDSAPublicKey, OKPPublicKey:
		var rawkey interface{}
		if err := key.Raw(&rawkey); err != nil {
			return "", nil, fmt.Errorf(`failed to get raw key from jwk.Key: %w`, err)
		}
		buf, err := x509.MarshalPKIXPublicKey(rawkey)
		if err != nil {
			return "", nil, fmt.Errorf(`failed to marshal PKIX: %w`, err)
		}
		return pmPublicKey, buf, nil
	default:
		return "", nil, fmt.Errorf(`unsupported key type %T`, key)
	}
}

// RegisterCustomField allows users to specify that a private field
// be decoded as an instance of the specified type. This option has
// a global effect.
//
// For example, suppose you have a custom field `x-birthday`, which
// you want to represent as a string formatted in RFC3339 in JSON,
// but want it back as `time.Time`.
//
// In that case you would register a custom field as follows
//
//	jwk.RegisterCustomField(`x-birthday`, timeT)
//
// Then `key.Get("x-birthday")` will still return an `interface{}`,
// but you can convert its type to `time.Time`
//
//	bdayif, _ := key.Get(`x-birthday`)
//	bday := bdayif.(time.Time)
func RegisterCustomField(name string, object interface{}) {
	registry.Register(name, object)
}

func AvailableCurves() []elliptic.Curve {
	return ecutil.AvailableCurves()
}

func CurveForAlgorithm(alg jwa.EllipticCurveAlgorithm) (elliptic.Curve, bool) {
	return ecutil.CurveForAlgorithm(alg)
}

// Equal compares two keys and returns true if they are equal. The comparison
// is solely done against the thumbprints of k1 and k2. It is possible for keys
// that have, for example, different key IDs, key usage, etc, to be considered equal.
func Equal(k1, k2 Key) bool {
	h := crypto.SHA256
	tp1, err := k1.Thumbprint(h)
	if err != nil {
		return false // can't report error
	}
	tp2, err := k2.Thumbprint(h)
	if err != nil {
		return false // can't report error
	}

	return bytes.Equal(tp1, tp2)
}

// IsPrivateKey returns true if the supplied key is a private key of an
// asymmetric key pair. The argument `k` must implement the `AsymmetricKey`
// interface.
//
// An error is returned if the supplied key is not an `AsymmetricKey`.
func IsPrivateKey(k Key) (bool, error) {
	asymmetric, ok := k.(AsymmetricKey)
	if ok {
		return asymmetric.IsPrivate(), nil
	}
	return false, fmt.Errorf("jwk.IsPrivateKey: %T is not an asymmetric key", k)
}

type keyValidationError struct {
	err error
}

func (e *keyValidationError) Error() string {
	return fmt.Sprintf(`key validation failed: %s`, e.err)
}

func (e *keyValidationError) Unwrap() error {
	return e.err
}

func (e *keyValidationError) Is(target error) bool {
	_, ok := target.(*keyValidationError)
	return ok
}

// NewKeyValidationError wraps the given error with an error that denotes
// `key.Validate()` has failed. This error type should ONLY be used as
// return value from the `Validate()` method.
func NewKeyValidationError(err error) error {
	return &keyValidationError{err: err}
}

func IsKeyValidationError(err error) bool {
	var kve keyValidationError
	return errors.Is(err, &kve)
}
