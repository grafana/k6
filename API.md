API signatures
==============

This file will be removed.

(global)
--------

* `require(mod)`
  
  Require a module.
  
  ```js
  var http = require('http');
  ```

* `sleep(t)`
  
  Sleep for `t` seconds. Decimals are allowed; `sleep(0.5)` sleeps for half a second.
  
  This may be moved somewhere else later on.

vu
--

Provides information about the currently executing VU.

* `vu.id()`
  
  Returns the VU's numeric ID.

http
----

* `http.do(method, url[, data[, params]])`
  
  Sends an HTTP Request.
  
  For GET requests, data is an object that's turned into query parameters. For any other kind, strings are sent verbatim, `null` and `undefined` omit the request body, and objects and arrays are JSON encoded.
  
  Params:
  
  * `quiet` - `bool`
    Do not report statistical information about this request.
  * `headers` - `object`
    Headers to set on the request. Values will be turned into strings if they're not already.
  
  Returns: `HTTPResponse` object.
  
  * `status` - `integer`
    HTTP status code.
  * `body` - `string`
    Response body. Always a string, empty if there's no body.
  * `headers` - `object`
    Response headers. May be empty if there are none, but it's always an object.
  * `json()`
    Decodes the response body into JSON.

* `http.get(...)` - Alias for `http.do('GET', ...)`

* `http.post(...)` - Alias for `http.do('POST', ...)`

* `http.put(...)` - Alias for `http.do('PUT', ...)`

* `http.delete(...)` - Alias for `http.do('DELETE', ...)`

* `http.patch(...)` - Alias for `http.do('PATCH', ...)`

* `http.options(...)` - Alias for `http.do('OPTIONS', ...)`

log
---

* `log.type(type, msg[, extra])`
  
  Writes out a log message.
  
  Type is one of `debug`, `info`, `warn` and `error`; messages to unknown channels will be ignored. Note that `debug` messages are only displayed when running k6 with `-v` (`--verbose`).
  
  Extra is an object of extra data to be provided along with the message. It's considered good practice to have the message a fixed string, and use extra data to provide context information, rather than concatenating it with the message.
  
  ```js
  var log = require('log');
  // Don't do this
  log.error("Something happened: " + error);
  // Do this instead
  log.error("Something happened", { error: error });
  ```
