/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTPBatchShort(t *testing.T) {
	if testing.Short() {
		return
	}

	snippet := `
	import { _assert } from "k6"
	import http from "k6/http"

	export default function() {
		let responses = http.batch([
			"http://httpbin.org/status/200",
			"http://httpbin.org/status/404",
			"http://httpbin.org/status/500"
		])
		_assert(responses[0].status === 200)
		_assert(responses[1].status === 404)
		_assert(responses[2].status === 500)
	}
	`

	assert.NoError(t, runSnippet(snippet))
}

func TestHTTPBatchLong(t *testing.T) {
	if testing.Short() {
		return
	}

	snippet := `
	import { _assert } from "k6"
	import http from "k6/http"

	export default function() {
		let responses = http.batch([
			"http://httpbin.org/status/200",
			"http://httpbin.org/status/404",
			{
				"method": "GET",
				"url": "http://httpbin.org/status/500",
				"body": null,
				"params": {}
			}
		])
		_assert(responses[0].status === 200)
		_assert(responses[1].status === 404)
		_assert(responses[2].status === 500)
	}
	`

	assert.NoError(t, runSnippet(snippet))
}

func TestHTTPBatchObject(t *testing.T) {
	if testing.Short() {
		return
	}

	snippet := `
	import { _assert } from "k6"
	import http from "k6/http"

	export default function() {
		let responses = http.batch({
			"twohundred": "http://httpbin.org/status/200",
			"fourohfour": "http://httpbin.org/status/404",
			"fivehundred": "http://httpbin.org/status/500"
		})
		_assert(responses.twohundred.status === 200)
		_assert(responses.fourohfour.status === 404)
		_assert(responses.fivehundred.status === 500)
	}
	`

	assert.NoError(t, runSnippet(snippet))
}
