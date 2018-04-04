TODO: Intro

## New Features!

### CLI/Options: Add `--tag` flag and `tags` option to set test-wide tags (#553)

You can now specify any number of tags on the command line using the `--tag NAME=VALUE` flag. You can also use the `tags` option to the set tags in the code.

The specified tags will be applied across all metrics. However if you have set a tag with the same name on a request, check or custom metric in the code that tag value will have precedence.

Thanks to @antekresic for their work on this!

**Docs**: [Test wide tags](https://docs.k6.io/v1.0/docs/tags-and-groups#section-test-wide-tags) and [Options](https://docs.k6.io/v1.0/docs/options#section-available-options)

### k6/http: Support for HTTP NTLM Authentication (#556)

```js
import http from "k6/http";
import { check } from "k6";

export default function() {
    // Passing username and password as part of URL plus the auth option will authenticate using HTTP Digest authentication
    let res = http.get("http://user:passwd@example.com/path", {auth: "ntlm"});

    // Verify response
    check(res, {
        "status is 200": (r) => r.status === 200
    });
}
```

**Docs**: [HTTP Params](http://k6.readme.io/docs/params-k6http)

## UX

* Clearer error message when using `open` function outside init context (#563)
* Better error message when a script or module can't be found (#565). Thanks to @antekresic for their work on this!

## Internals

* Removed all httpbin.org usage in tests, now a local transient HTTP server is used instead (#555). Thanks to @mccutchen for the great [go-httpbin](https://github.com/mccutchen/go-httpbin) library!
* Fixed various data races and enabled automated testing with `-race` (#564)

## Bugs
* Archive: archives generated on Windows can now run on *nix and vice versa. (#566)
