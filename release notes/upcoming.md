TODO: Intro

## New Features!

### New option: No Cookies Reset (#729)

A new option has been added that disables the default behavior of resetting the [cookie jar](https://docs.k6.io/docs/cookies) after each VU iteration. If it's enabled, saved cookies will be persisted across VUs iterations. For the moment there's no CLI flag for this option, instead it can only be set via the `noCookiesReset` key from the exported script `options` or via the `K6_NO_COOKIES_RESET` environment variable.

### k6/http: Ability to discard response bodies (#742)

Saving HTTP response body data by default is useful, especially when we need to parse it to obtain information that is necessary for making subsequent requests. But in many cases it makes little to no sense to spend memory on saving the response body. For example, when requesting static website assets (JS, CSS, images etc.), the actual file contents rarely matter when running load tests.

For cases like that, a new option has been added that allows k6 to discard incoming data on arrival, in order to save CPU cycles and prevent unnecessary copying of data. When enabled, the actual HTTP response body would be fully downloaded (so that the load test and all HTTP metrics for that request are still accurate), it just won't be saved in memory and passed on to the JS runtime - the `response.body` would be empty.

This new `discardResponseBody` option can be specified both on a per-request basis like this:

```js
import http from 'k6/http';

export default function () {
  // Do something with the response...
  let response = http.get("http://test.loadimpact.com");

  // Ignore static files
  http.get("http://test.loadimpact.com/images/logo.png", { discardResponseBody: true });
  http.get("http://test.loadimpact.com/style.css", { discardResponseBody: true });
  // ...
};
```

Or it can also be specified globally, and individual responses can be exempted from it. The global option can be changed via the `--discard-response-body` CLI flag, the `K6_DISCARD_RESPONSE_BODY` environment variable or the `discardResponseBody` script option:
```js
import http from 'k6/http';

export let options = {
  discardResponseBody: true,
};

export default function () {
  // Do something with the response...
  let response = http.get("http://test.loadimpact.com", { discardResponseBody: false });

  // Ignore static files
  http.batch([
    "http://test.loadimpact.com/images/logo.png",
    "http://test.loadimpact.com/style.css"
  ]);
};
```

Thanks to @ofauchon for their work on this!


## Internals

 * Cloud output: improved outlier metric detection for small batches (#744)

## Bugs fixed!

* UI: The interactive `k6 login influxdb` command failed to write the supplied options to the config file. (#734)
* UI: Password input is now masked in `k6 login influxdb` and `k6 login cloud`. (#734)
* Config: Environment variables can now be used to modify k6's behavior in the `k6 login` subcommands. (#734)