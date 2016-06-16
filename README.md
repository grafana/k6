Speedboat
=========

Speedboat is the codename for the next generation of Load Impact's load generator.

Installation
------------

### On your own machine

```
go get github.com/loadimpact/speedboat
```

Make sure you have Go 1.6 or later installed. It will be installed to `$GOPATH/bin`, so you probably want that in your PATH.

### Using Docker

```
docker pull loadimpact/speedboat
```

You can now run speedboat using `docker run loadimpact/speedboat [...]`.

Substitute the `speedboat` command for this in the instructions below if using this method.

Running (standalone)
--------------------

```
speedboat run --script scripts/google.js
```

This will run a very simple load test against `https://google.com/` for 10 seconds (change with eg `-d 15s` or `-d 2m`), using 2 VUs (change with eg `-u 4`).

Running (distributed)
---------------------

```
# On the master machine
speedboat master -h 0.0.0.0

# On each worker machine
speedboat worker -m master.local

# On the client machine
speedboat run -m master.local --script scripts/google.js
```

This will run a distributed test, on any number of machines in parallel, using `master.local` as a central point for communication. The master's firewall must allow access to the ports `9595` and `9596`.

Scripting API
-------------

This describes the JS scripting API, as it is currently implemented.

#### Global functions (no need to import these) :

##### sleep(t)

Makes the VU thread sleep a specified amount of time.

| Argument | Type    | Description |
| -------- | :-----: | ----------: |
| t        | float   | Number of seconds to sleep

Returns: none

#### Module "test" (require "test") :

##### test.abort()

Aborts the whole test.

Arguments: none

Returns: none

##### test.url()

Returns the value given to the --url command line parameter (string)

Arguments: none

#### Module "console" (require "console") :

##### console.log(s)

Logs the supplied string "s" to the console, using a "LOG" subject/header.

| Argument | Type    | Description |
| -------- | :-----: | ----------: |
| s        | string  | string to output

Returns: none

##### console.warn(s)

Logs the supplied string "s" to the console, using a "WARN" subject/header.

| Argument | Type    | Description |
| -------- | :-----: | ----------: |
| s        | string  | string to output

Returns: none

##### console.err(s)

Logs the supplied string "s" to the console, using a "ERR" subject/header.

| Argument | Type    | Description |
| -------- | :-----: | ----------: |
| s        | string  | string to output

Returns: none

#### Module "vu" (require "vu") :

##### vu.id()

Returns the unique ID number of the currently executing VU (integer).

Arguments: none

##### vu.iteration()

Returns the sequence number of the currently executing script iteration (integer).

Arguments: none

#### Module "http" (require "http") :

##### http.request(method, url, body, args)

| Argument | Type        | Description |
| -------- | :---------: | ----------: |
| method   | string      | Which HTTP method to use ("GET", "POST", "HEAD", "PUT" and "DELETE" supported
| url      | string      | Which URL to get
| body     | string      | HTTP request body (mostly used for POSTs)
| args     | RequestArgs | Struct/map that can currently hold two boolean parameters ("report" and "follow"), and a string parameter ("userAgent"), see below.

| Return value | Type   | Description |
| ------------ | :----: | ----------: |
| result       | Result | Struct/map that contains four items: "Text", "Time", "Error" and "Abort", see below.

This function is the primitive that all the other http.method functions make use of to generate HTTP requests. It can take a RequestArgs struct as parameter, which has the following layout:

| Field name | Type    | Description |
| ---------- | :-----: | ----------: |
| follow     | boolean | Defines whether Speedboat should follow HTTP redirects, or not. Defaults to true.
| report     | boolean | Defines whether Speedboat should report results, or not. Defaults to true.
| userAgent  | string  | Defines what User-Agent string Speedboat should send to the target host.

The Result struct returned from a call has the following fields:

| Field name | Type          | Description |
| ---------- | :-----------: | ----------: |
| Text       | string        | The HTTP body data sent by the remote server.
| Time       | time.Duration | How long the request took. See https://golang.org/pkg/time/#Duration
| Error      | interface     | See https://golang.org/pkg/builtin/#error
| Abort      | boolean       | If the test was aborted?

##### http.get(url, args)

See description of http.request() above.

##### http.head(url, args)

See description of http.request() above.

##### http.post(url, body, args)

See description of http.request() above.

##### http.put(url, body, args)

See description of http.request() above.

##### http.delete(url, body, args)

See description of http.request() above.

##### http.setMaxConnsPerHost(n)

| Argument | Type    | Description |
| -------- | :-----: | ----------: |
| n        | integer | Max # of concurrent connections each VU can use per target host it communicates with.

This setting is used to emulate browser behaviour better, where browsers tend to use multiple, concurrent connections when fetching >1 resource from the same host. Most modern browsers use 4-8 concurrent connections per host, by default.


Planned additions to scripting API
----------------------------------

- dns.remap()
- dns.lookup()
- html support using goquery
- http.auto_cookie_handling
- http.force_sslv1/v2/v3
- http.force_tlsv1/v2
- xml.?


