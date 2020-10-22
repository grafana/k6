/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
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

package js

import (
	// Initialize all internal JS modules.
	_ "github.com/loadimpact/k6/js/modules/k6"
	_ "github.com/loadimpact/k6/js/modules/k6/crypto"
	_ "github.com/loadimpact/k6/js/modules/k6/crypto/x509"
	_ "github.com/loadimpact/k6/js/modules/k6/encoding"
	_ "github.com/loadimpact/k6/js/modules/k6/grpc"
	_ "github.com/loadimpact/k6/js/modules/k6/http"
	_ "github.com/loadimpact/k6/js/modules/k6/metrics"
	_ "github.com/loadimpact/k6/js/modules/k6/ws"
)
