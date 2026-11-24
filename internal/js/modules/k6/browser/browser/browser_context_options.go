package browser

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"reflect"

	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/clientcert"
	k6common "go.k6.io/k6/v2/js/common"
)

// clientCertificateEntry is one clientCertificates option entry with its fields converted.
type clientCertificateEntry struct {
	origin     string
	cert, key  []byte
	passphrase string
}

// parseClientCertificates validates and loads the clientCertificates browser context option.
// The result is keyed by normalized origin; each origin lists its entries in option order.
func parseClientCertificates(rt *sobek.Runtime, opts sobek.Value) (map[string][]tls.Certificate, error) {
	if k6common.IsNullish(opts) {
		return nil, nil //nolint:nilnil
	}
	arr, ok := opts.(*sobek.Object)
	if !ok || arr.ClassName() != "Array" {
		return nil, errors.New("clientCertificates must be an array")
	}

	n := int(arr.Get("length").ToInteger())
	entries := make([]clientCertificateEntry, 0, n)
	for i := range n {
		e, err := parseClientCertificateEntry(rt, i, arr.Get(fmt.Sprint(i)))
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}

	var certs map[string][]tls.Certificate
	for i, e := range entries {
		c, err := clientcert.LoadCertificate(e.cert, e.key, e.passphrase)
		if err != nil {
			return nil, fmt.Errorf("clientCertificates[%d]: failed to load client certificate: %w", i, err)
		}
		for _, existing := range certs[e.origin] {
			if existing.Leaf.PublicKeyAlgorithm == c.Leaf.PublicKeyAlgorithm {
				return nil, fmt.Errorf(
					"clientCertificates[%d]: more than one %s certificate for origin %s; "+
						"only one per key type is supported",
					i, c.Leaf.PublicKeyAlgorithm, e.origin)
			}
		}
		if certs == nil {
			certs = make(map[string][]tls.Certificate)
		}
		certs[e.origin] = append(certs[e.origin], c)
	}
	return certs, nil
}

func parseClientCertificateEntry(rt *sobek.Runtime, i int, v sobek.Value) (clientCertificateEntry, error) {
	var e clientCertificateEntry
	obj, ok := v.(*sobek.Object)
	if k6common.IsNullish(v) || !ok {
		return e, fmt.Errorf("clientCertificates[%d]: expected object", i)
	}
	// Playwright's certPath/keyPath/pfxPath read files at newContext time, which bypasses
	// open() and so k6 archives; reject them instead of silently ignoring them.
	for _, f := range []struct{ path, field string }{
		{"certPath", "cert"}, {"keyPath", "key"}, {"pfxPath", "pfx"},
	} {
		if !k6common.IsNullish(obj.Get(f.path)) {
			return e, fmt.Errorf("clientCertificates[%d].%s: file paths are not supported; "+
				"read the file with open() in the init context and pass its contents as %s", i, f.path, f.field)
		}
	}
	if !k6common.IsNullish(obj.Get("pfx")) {
		return e, fmt.Errorf("clientCertificates[%d].pfx: PFX is not supported yet; pass cert and key as PEM", i)
	}

	var (
		err           error
		hasOrigin     bool
		hasPassphrase bool
	)
	if e.origin, hasOrigin, err = clientCertificateString(obj.Get("origin")); err != nil {
		return e, fmt.Errorf("clientCertificates[%d].origin: %w", i, err)
	}
	if e.cert, err = clientCertificateBytes(rt, obj.Get("cert")); err != nil {
		return e, fmt.Errorf("clientCertificates[%d].cert: %w", i, err)
	}
	if e.key, err = clientCertificateBytes(rt, obj.Get("key")); err != nil {
		return e, fmt.Errorf("clientCertificates[%d].key: %w", i, err)
	}
	if e.passphrase, hasPassphrase, err = clientCertificateString(obj.Get("passphrase")); err != nil {
		return e, fmt.Errorf("clientCertificates[%d].passphrase: %w", i, err)
	}

	hasCert, hasKey := e.cert != nil, e.key != nil
	switch {
	case !hasOrigin:
		err = errors.New("clientCertificates.origin is required")
	case !hasCert && !hasKey && !hasPassphrase:
		err = errors.New("None of cert, key, passphrase or pfx is specified") //nolint:staticcheck // matches Playwright
	case hasCert && !hasKey:
		err = errors.New("cert is specified without key")
	case !hasCert && hasKey:
		err = errors.New("key is specified without cert")
	case !hasCert:
		err = errors.New("failed to load client certificate: cert and key, or pfx, are required")
	}
	if err == nil {
		e.origin, err = clientcert.NormalizeOrigin(e.origin)
	}
	if err != nil {
		return e, fmt.Errorf("clientCertificates[%d]: %w", i, err)
	}
	return e, nil
}

// clientCertificateString converts an optional string field and reports whether it was set.
func clientCertificateString(v sobek.Value) (string, bool, error) {
	if v == nil || sobek.IsUndefined(v) {
		return "", false, nil
	}
	s, ok := v.Export().(string)
	if !ok {
		return "", false, errors.New("expected string")
	}
	return s, true, nil
}

// typedArrayExportTypes are the Go types that only TypedArrays export as.
//
//nolint:gochecknoglobals // read-only lookup table of reflect types
var typedArrayExportTypes = map[reflect.Type]bool{
	reflect.TypeFor[[]byte]():    true,
	reflect.TypeFor[[]int8]():    true,
	reflect.TypeFor[[]uint16]():  true,
	reflect.TypeFor[[]int16]():   true,
	reflect.TypeFor[[]uint32]():  true,
	reflect.TypeFor[[]int32]():   true,
	reflect.TypeFor[[]float32](): true,
	reflect.TypeFor[[]float64](): true,
	reflect.TypeFor[[]int64]():   true,
	reflect.TypeFor[[]uint64]():  true,
}

// clientCertificateBytes converts an optional string, ArrayBuffer or TypedArray field to a copy
// of its bytes; nil means the field was not set. The type is recognised by its Go export type,
// which scripts cannot forge.
func clientCertificateBytes(rt *sobek.Runtime, v sobek.Value) ([]byte, error) {
	if v == nil || sobek.IsUndefined(v) {
		return nil, nil
	}
	switch exp := v.Export().(type) {
	case string:
		return []byte(exp), nil
	case sobek.ArrayBuffer:
		return bytes.Clone(exp.Bytes()), nil
	}
	if _, isObj := v.(*sobek.Object); isObj && typedArrayExportTypes[v.ExportType()] {
		var b []byte
		if err := rt.ExportTo(v, &b); err != nil {
			return nil, fmt.Errorf("expected string, ArrayBuffer or TypedArray: %w", err)
		}
		return bytes.Clone(b), nil
	}
	return nil, errors.New("expected string, ArrayBuffer or TypedArray")
}
