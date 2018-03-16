TODO: Intro

## New Features!

### k6/http: Support for binary files and multipart requests (#370, #420 and #524)
The init context [`open()`](https://docs.k6.io/docs/open-filepath) function now supports binary files:

```js
import http from "k6/http";
import {md5} from "k6/crypto";

let binFile = open("./image.png", "b");

export default function() {
    console.log(md5(binFile, "hex"));
}
```

and the HTTP module has handily gained support for multipart requests:

```js
import http from "k6/http";

let binFile = open("./image.png", "b");

export default function() {
    var data = {
        field: "this is a standard form field",
        file: http.file(binFile, "my image file.png")
    };
    var res = http.post("https://example.com/upload", data);
}
```

Thanks @dstpierre for their work on this!

**Docs**: [Multipart requests](https://docs.k6.io/docs/TODO)

### k6/http: Request information through response object (#447)
Request information is now exposed through the [Response object](https://docs.k6.io/docs/response-k6http):

```js
import http from "k6/http";

export default function() {
    let res = http.get("https://example.com/")
    console.log(`Method: ${res.request.method}`);
    new Map(Object.entries(res.request.headers)).forEach((v, k) => console.log(`Header: ${k}=${v}`));
    console.log(`Body: ${res.request.method}`);
}
```

Thanks to @cstyan for their work on this!

**Docs**: [Request information](https://docs.k6.io/docs/TODO)

### Lifecycle: setup/teardown functions (#457)
Finally k6 has the same basic test lifecycle hooks as many "normal" testing tools, setup and teardown, and you have the full JS API of k6 available within these functions which means you can make HTTP calls etc. that you can’t do in the global/init scope.

To use the lifecycle hooks you simply define an exported setup() and/or teardown() function in your script:

```js
export function setup() {
	return { “data”: “passed to main and teardown function” };
}

export function teardown(data)  {
	console.log(JSON.stringify(data));
}

export default function(data) {
if (data.v != 1) {
		throw new Error("incorrect data: " + JSON.stringify(data));
	}
}
```

**Docs**: [Test life cycle](https://docs.k6.io/v1.0/docs/test-life-cycle)

### CLI: HTTP debug flag (#447)
If you specify `--http-debug` when running a test k6 will now continuously print request and response information.

Thanks to @marklagendijk for their work on this!

**Docs**: [HTTP debugging](https://docs.k6.io/docs/TODO)


### Options: DNS override (#494)
Overriding DNS resoution of hostnames can come in handy when testing a system that is run in multiple environments (dev, staging, prod etc.) with different IP addresses but responds to the same `Host` header.

```js
import http from "k6/http";

export let options = {
    hosts: {
        "loadimpact.com": "1.2.3.4",
        "test.loadimpact.com": "5.6.7.8"
    }
};

export default function() {
    http.get("http://loadimpact.com/");
    http.get("http://test.loadimpact.com/");
}
```

Tip: you can use [environment variables]() to switch the IP based on environment.

Thanks @luizbafilho for their work on this!

**Docs**: [DNS Override](https://docs.k6.io/docs/TODO)

### CLI: Add `-e` flag environment variable flag (#495)

You can now specify any number of environment variables on the command line using the `-e NAME=VALUE` flag.

As a matter of security, when running `k6 cloud ...` or `k6 archive ...` the system's environment variables will not be included in the resulting archive, you'll now have to use the new `--include-system-env-vars` flag to get that behavior. When executing `k6 run ...` the system's environment will continue to be exposed to your script.

We encourage the use of `-e NAME=VALUE` to make environment variable use explicit and compatible across local and cloud execution.

Thanks @na-- for their work on this!

**Docs**: [Environment variables](https://docs.k6.io/v1.0/docs/environment-variables)

### HAR converter: Add `--no-batch` flag (#497)

A `--no-batch` CLI flag has been added to `k6 convert` command to disable the creation of batch request statements in favor of individual `http.get/del/options/patch/post/put` statements.

Thanks @danron and @cyberw for their work on this!


### HAR converter: Add `--return-on-failed-check` flag (#499)

A `--return-on-failed-check` CLI flag has been added to `k6 convert` command to optionally return/exit the current VU iteration if a response status code check fails (requires the existing `--enable-status-code-checks` to be specified as well).

Thanks @cyberw for their work on this!

### HAR converter: Add `--correlate` flag (#500)

A first step towards doing correlations when converting HAR to JS has implemented. In this first iteration, if `--correlate` is specified the converter will try to detect issues with redirects.

Thanks @cyberw for their work on this!

### Stats: Use linear interpolation when calculating percentiles (#498)

The percentile calculation has been changed to use linear interpolation of two bounding values if percentile doesn't precisely fall on a value/sample index.

### Thresholds: Support for aborting test early as soon as threshold is hit (#508)

Up until now thresholds were evaluated continuously throughout the test but could never abort a running test.
This PR adds functionality to specify that a test run should abort the test as soon as a threshold evaluates to false, optionally with a delay in threshold evaluation to avoid aborting to early when the number of samples collected is low.

```js
export let options = {
    thresholds: {
        "http_req_duration": ["avg<100", { threshold: "p(95)<200", abortOnFail: true, delayAbortEval: "30s" }]
    }
};
```

Thanks @antekresic for their work on this!

**Docs**: [Thresholds](https://docs.k6.io/docs/thresholds)

### Docker: Use full Alpine as base image for k6 (#514)

Thanks @pkruhlei for their contribution!

### CLI: Option to whitelist what tags should be added to metric samples (#525)

Adds a CLI option `--default-tags "url,method,status"` to specify which tags to include in metrics output, a whitelist. Collectors have the possibility to override and force certain tags to be included.

The following tags can be specified:

- `error` (http)
- `group` (http)
- `iter` (vu)
- `method` (http)
- `name` (http)
- `ocsp_status` (http)
- `proto` (http)
- `status` (http, websocket)
- `subprotocol` (websocket)
- `tls_version` (http)
- `url` (http, websocket)
- `vu` (vu)

**Docs**: [System tags](https://docs.k6.io/v1.0/docs/tags-and-groups#section-system-tags)

## Bugs fixed!

* HAR converter: Fixed issue wuth construction of `body` parameter when `PostData.Params` values are present. (#489)

* Stats: Fixed output of rate metrics to truncate rather than round when converting to string representation from float for summary output.

* Stats: Fixes issue where calls to `TrendSink.P()` and `TrendSink.Format()` could return wrong results if `TrendSink.Calc()` hadn't been previously called. (#498)

* Cloud/Insights: Fixed issue causing default test name to be empty when parsing script from STDIN (#510)

* Cloud/Insights: Fixed handling of unexpected responses from server. (#522)

* Stats: Fixed issue with calculation of `data_received` and `data_sent` metrics. (#523)
