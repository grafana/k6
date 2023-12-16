// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package qtls

func needFIPS() bool { return false }

func supportedSignatureAlgorithms() []SignatureScheme {
	return defaultSupportedSignatureAlgorithms
}

func fipsMinVersion(c *config) uint16          { panic("fipsMinVersion") }
func fipsMaxVersion(c *config) uint16          { panic("fipsMaxVersion") }
func fipsCurvePreferences(c *config) []CurveID { panic("fipsCurvePreferences") }
func fipsCipherSuites(c *config) []uint16      { panic("fipsCipherSuites") }

var fipsSupportedSignatureAlgorithms []SignatureScheme
