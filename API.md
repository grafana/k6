API signatures
==============

This file will be removed.

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

* `http.get(...)` - Alias for `http.do('GET', ...)`

* `http.post(...)` - Alias for `http.do('POST', ...)`

* `http.put(...)` - Alias for `http.do('PUT', ...)`

* `http.delete(...)` - Alias for `http.do('DELETE', ...)`
