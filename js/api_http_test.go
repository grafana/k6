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

func TestHTTPBatchError(t *testing.T) {
	if testing.Short() {
		return
	}

	snippet := `
	import http from "k6/http"
	export default function() { http.batch([""]) }
	`

	assert.EqualError(t, runSnippet(snippet), "Error: Get : unsupported protocol scheme \"\"")
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

func TestHTTPFormURLEncodeHeader(t *testing.T) {
	if testing.Short() {
		return
	}

	snippet := `
	import { _assert } from "k6"
	import http from "k6/http"

	export default function() {
		let response = http.post("http://httpbin.org/post", { field: "value" })
		_assert(response.json()["form"].hasOwnProperty("field"))
		_assert(response.json()["form"]["field"] === "value")
	}
	`

	assert.NoError(t, runSnippet(snippet))
}

func TestHTTPFormURLEncodeList(t *testing.T) {
	if testing.Short() {
		return
	}

	snippet := `
	import { _assert } from "k6"
	import http from "k6/http"

	export default function() {
		let response = http.post("http://httpbin.org/post", { field: ["value1", "value2", "value3"] }, { headers: {"Content-Type": "application/x-www-form-urlencoded"} })
		_assert(response.json()["form"].hasOwnProperty("field"))
		_assert(JSON.stringify(response.json()["form"]["field"]) === JSON.stringify(["value1", "value2", "value3"]))
	}
	`

	assert.NoError(t, runSnippet(snippet))
}

func TestHTTPCookiesAccess(t *testing.T) {
	if testing.Short() {
		return
	}

	snippet := `
	import { _assert } from "k6"
	import http from "k6/http"

	export default function() {
        let response = http.get("http://httpbin.org/cookies", { cookies: { key: "value" } });
		_assert(response.cookies["key"] === "value")
	}
	`

	assert.NoError(t, runSnippet(snippet))
}

func TestHTTPCookiesAccessNoCookie(t *testing.T) {
	if testing.Short() {
		return
	}

	snippet := `
	import { _assert } from "k6"
	import http from "k6/http"

	export default function() {
        let response = http.get("http://httpbin.org/");
		_assert(Object.keys(response.cookies).length === 0)
	}
	`

	assert.NoError(t, runSnippet(snippet))
}

func TestHTTPCookiesSetSimple(t *testing.T) {
	if testing.Short() {
		return
	}

	snippet := `
	import { _assert } from "k6"
	import http from "k6/http"

	export default function() {
        let response = http.get("http://httpbin.org/cookies", { cookies: { key: "value", key2: "value2" } });
		_assert(response.cookies["key"] === "value")
		_assert(response.cookies["key2"] === "value2")
	}
	`

	assert.NoError(t, runSnippet(snippet))
}

func TestHTTPCookiesSetSimpleWithHeaderOverride(t *testing.T) {
	if testing.Short() {
		return
	}

	snippet := `
	import { _assert } from "k6"
	import http from "k6/http"

	export default function() {
		let cookies = { key: "value", key2: "value2" };
		let headers = { "Cookie": "key=overridden-value; key2=overridden-value2" }
        let response = http.get("http://httpbin.org/cookies", { cookies: cookies, headers: headers });

		// Cookies added through Cookie headers are not added to the jar...
		_assert(response.cookies["key"] === "value")
		_assert(response.cookies["key2"] === "value2")

		// ...but they override the actual values being sent to the server
		_assert(response.json().cookies.key === "overridden-value")
		_assert(response.json().cookies.key2 === "overridden-value2")
	}
	`

	assert.NoError(t, runSnippet(snippet))
}

func TestHTTPCookiesCookieJar(t *testing.T) {
	if testing.Short() {
		return
	}

	snippet := `
	import { _assert } from "k6"
	import http from "k6/http"

	export default function() {
		let jar = new http.CookieJar();

		// Date to RFC 1123
		_assert(http.CookieJar.dateToRFC1123(new Date("Sun, 24 Jul 1983 17:01:02 GMT")) === "Sun, 24 Jul 1983 17:01:02 GMT", "dateToRFC1123 failed")

		// Empty cookie jar
		_assert(jar.cookies.length === 0)

		// Set cookie without params
		jar.set("key", "value");
		_assert(jar.cookies[0]["key"] === "key")
		_assert(jar.cookies[0]["value"] === "value")

		// Set cookie with params
		jar.set("key", "value", { domain: "example.com", path: "/", expires: "Sun, 24 Jul 1983 17:01:02 GMT", maxAge: 123456789, secure: true, httpOnly: true });
		_assert(jar.cookies[1]["key"] === "key")
		_assert(jar.cookies[1]["value"] === "value")
		_assert(jar.cookies[1]["domain"] === "example.com")
		_assert(jar.cookies[1]["path"] === "/")
		_assert(jar.cookies[1]["expires"] === "Sun, 24 Jul 1983 17:01:02 GMT")
		_assert(jar.cookies[1]["maxAge"] === 123456789)
		_assert(jar.cookies[1]["secure"] === true)
		_assert(jar.cookies[1]["httpOnly"] === true)

		// Set cookie with Date expires
		jar.set("key", "value", { expires: new Date("Sun, 24 Jul 1983 17:01:02 GMT") });
		_assert(jar.cookies[2]["key"] === "key")
		_assert(jar.cookies[2]["value"] === "value")
		_assert(jar.cookies[2]["expires"] === "Sun, 24 Jul 1983 17:01:02 GMT", "cookie RFC1123 failed")
	}
	`

	assert.NoError(t, runSnippet(snippet))
}

func TestHTTPCookiesSetCookieJar(t *testing.T) {
	if testing.Short() {
		return
	}

	snippet := `
	import { _assert } from "k6"
	import http from "k6/http"

	export default function() {
		let jar = new http.CookieJar();
		jar.set("key", "value");
		jar.set("key2", "value2");
        let response = http.get("http://httpbin.org/cookies", { cookies: jar });
		_assert(response.cookies["key"] === "value")
		_assert(response.cookies["key2"] === "value2")
	}
	`

	assert.NoError(t, runSnippet(snippet))
}

func TestHTTPCookiesSetCookieJarDomain(t *testing.T) {
	if testing.Short() {
		return
	}

	snippet := `
	import { _assert } from "k6"
	import http from "k6/http"

	export default function() {
		let jar = new http.CookieJar();

		// Matching domain
		jar.set("key", "value", { domain: "httpbin.org" });
        let response = http.get("http://httpbin.org/cookies", { cookies: jar });
		_assert(response.cookies["key"] === "value")

		// Mismatching domain
		jar.set("key2", "value2", { domain: "example.com" });
        response = http.get("http://httpbin.org/cookies", { cookies: jar });
		_assert(response.cookies["key"] === "value")
		_assert(response.cookies["key2"] === undefined)
	}
	`

	assert.NoError(t, runSnippet(snippet))
}

func TestHTTPCookiesSetCookieJarPath(t *testing.T) {
	if testing.Short() {
		return
	}

	snippet := `
	import { _assert } from "k6"
	import http from "k6/http"

	export default function() {
		let jar = new http.CookieJar();

		// Matching path
		jar.set("key", "value", { path: "/cookies" });
        let response = http.get("http://httpbin.org/cookies", { cookies: jar });
		_assert(response.cookies["key"] === "value", "cookie 'key' not found")

		// Mismatching path
		jar.set("key2", "value2", { path: "/some-other-path" });
        response = http.get("http://httpbin.org/cookies", { cookies: jar });
		_assert(response.cookies["key"] === "value", "cookie 'key' not found")
		_assert(response.cookies["key2"] === undefined, "cookie 'key2' unexpectedly found")
	}
	`

	assert.NoError(t, runSnippet(snippet))
}

func TestHTTPCookiesSetCookieJarExpires(t *testing.T) {
	if testing.Short() {
		return
	}

	snippet := `
	import { _assert } from "k6"
	import http from "k6/http"

	export default function() {
		let jar = new http.CookieJar();

		// Cookie with expiry date in past
		jar.set("key", "value", { expires: "Sun, 24 Jul 1983 17:01:02 GMT" });
        let response = http.get("https://httpbin.org/cookies", { cookies: jar });
		_assert(response.cookies["key"] === undefined, "cookie 'key' unexpectedly found")

		// Cookie with expiry date in future
		jar.set("key", "value", { expires: "Sat, 24 Jul 2083 17:01:02 GMT" });
        response = http.get("https://httpbin.org/cookies", { cookies: jar });
		_assert(response.cookies["key"] === "value", "cookie 'key' not found")
	}
	`

	assert.NoError(t, runSnippet(snippet))
}

func TestHTTPCookiesSetCookieJarSecure(t *testing.T) {
	if testing.Short() {
		return
	}

	snippet := `
	import { _assert } from "k6"
	import http from "k6/http"

	export default function() {
		let jar = new http.CookieJar();

		// Secure cookie sent over TLS connection
		jar.set("key", "value", { secure: true });
        let response = http.get("https://httpbin.org/cookies", { cookies: jar });
		_assert(response.cookies["key"] === "value", "cookie 'key' not found")

		// Secure cookie NOT sent over non-TLS connection
        response = http.get("http://httpbin.org/cookies", { cookies: jar });
		_assert(response.cookies["key"] === undefined, "cookie 'key' unexpectedly found")
	}
	`

	assert.NoError(t, runSnippet(snippet))
}
