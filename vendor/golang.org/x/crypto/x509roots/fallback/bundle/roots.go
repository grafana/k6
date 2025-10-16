// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package bundle contains the bundle of root certificates parsed from the NSS
// trust store, using x509roots/nss.
package bundle

import (
	"crypto/x509"
	_ "embed"
	"fmt"
	"iter"
	"time"
)

//go:embed bundle.der
var rawCerts []byte

// Root represents a root certificate parsed from the NSS trust store.
type Root struct {
	// Certificate is the DER-encoded certificate (read-only; do not modify!).
	Certificate []byte

	// Constraint is nil if the root is unconstrained. If Constraint is non-nil,
	// the certificate has additional constraints that cannot be encoded in
	// X.509, and when building a certificate chain anchored with this root the
	// chain should be passed to this function to check its validity. If using a
	// [crypto/x509.CertPool] the root should be added using
	// [crypto/x509.CertPool.AddCertWithConstraint].
	Constraint func([]*x509.Certificate) error
}

// Roots returns the bundle of root certificates from the NSS trust store. The
// [Root.Certificate] slice must be treated as read-only and should not be
// modified.
func Roots() iter.Seq[Root] {
	return func(yield func(Root) bool) {
		for _, unparsed := range unparsedCertificates {
			root := Root{
				Certificate: rawCerts[unparsed.certStartOff : unparsed.certStartOff+unparsed.certLength],
			}
			// parse possible constraints, this should check all fields of unparsedCertificate.
			if unparsed.distrustAfter != "" {
				distrustAfter, err := time.Parse(time.RFC3339, unparsed.distrustAfter)
				if err != nil {
					panic(fmt.Sprintf("failed to parse distrustAfter %q: %s", unparsed.distrustAfter, err))
				}
				root.Constraint = func(chain []*x509.Certificate) error {
					for _, c := range chain {
						if c.NotBefore.After(distrustAfter) {
							return fmt.Errorf("certificate issued after distrust-after date %q", distrustAfter)
						}
					}
					return nil
				}
			}
			if !yield(root) {
				return
			}
		}
	}
}

type unparsedCertificate struct {
	cn           string
	sha256Hash   string
	certStartOff int
	certLength   int

	// possible constraints
	distrustAfter string
}
