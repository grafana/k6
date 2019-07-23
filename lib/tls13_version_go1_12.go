// +build go1.12

/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package lib

import "crypto/tls"

// TLSVersion13 represents tls1.3 version supports.
const TLSVersion13 = tls.VersionTLS13

// TLS 1.3 cipher suites.
//nolint: golint
const (
	TLS13_CIPHER_SUITE_TLS_AES_128_GCM_SHA256       = tls.TLS_AES_128_GCM_SHA256
	TLS13_CIPHER_SUITE_TLS_AES_256_GCM_SHA384       = tls.TLS_AES_256_GCM_SHA384
	TLS13_CIPHER_SUITE_TLS_CHACHA20_POLY1305_SHA256 = tls.TLS_CHACHA20_POLY1305_SHA256
)
