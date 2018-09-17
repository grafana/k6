TODO: Intro

## New Features!

### New option: No Cookies Reset (#729)

A new option has been added that disables the default behavior of resetting the [cookie jar](https://docs.k6.io/docs/cookies) after each VU iteration. If it's enabled, saved cookies will be persisted across VU iterations. For the moment there's no CLI flag for this option, instead it can only be set via the `noCookiesReset` key from the exported script `options` or via the `K6_NO_COOKIES_RESET` environment variable.

### k6/http: New options to discard the response body or to specify its type (#742 and #749)

You can now specify what the type of an HTTP response's body should be with the new `responseType` request option. The possible values for it are `text` (the default), `binary` and `none`. The default `text` response type is backward-compatible, it doesn't change the current k6 behavior of returning the `body` attribute of the `http/Response` object as a string. It's well suited for working with web pages, text-based APIs and similar HTTP responses, but it can be unsuitable when dealing with binary files.

That's mostly because JavaScript strings are encoded with UTF-16 and converting binary data to it will frequently mangle some of the data. The new `binary` response type allows us to avoid that, it causes k6 to return the HTTP response's `body` as a byte array. This allows us to deal with the binary data without mangling it:
```js
import http from 'k6/http';
import { sha256 } from 'k6/crypto';

export default function () {
    const expectedLogoHash = "fce7a09dde7c25b9822eca8438b7a5c397c2709e280e8e50f04d98bc8a66f4d9";

    let resp = http.get("http://test.loadimpact.com/images/logo.png", { responseType: "binary" });
    let logoHash = sha256(resp.body, "hex");
    if (logoHash !== expectedLogoHash) {
        throw new Error(`Expected logo hash to be ${expectedLogoHash} but it was ${logoHash}`);
    }
    http.post("https://httpbin.org/post", resp.body);
};
```

Saving HTTP response bodies is generally useful, especially when we need to use them (or parts of them) in subsequent requests. But in many cases it makes little to no sense to spend memory on saving the response body. For example, when requesting static website assets (JS, CSS, images etc.) or web pages without needed information, the actual file contents rarely matter when running load tests.

For cases like that, the value `none` for the `responseType` option allows k6 to discard incoming data on arrival, in order to save CPU cycles and prevent unnecessary copying of data. When enabled, the actual HTTP response body would be fully downloaded (so that the load test and all HTTP metrics for that request are still accurate), it just won't be saved in memory and passed on to the JavaScript runtime at all - the `response.body` attribute would be `null`:

```js
import http from 'k6/http';
import { check } from "k6";

export default function () {
    const url = "http://test.loadimpact.com";
    let resp = http.get(url);
    let cssFile = resp.html().find("link[rel='stylesheet']").attr("href");

    check(http.get(`${url}/${cssFile}`, { responseType: "none" }), {
        "body was empty": (res) => res.body === null,
        "response code was 200": (res) => res.status == 200,
        "timings are present": (res) => res.timings.duration > 0,
    });
};
```

For convenience, there's also a new global config option that causes k6 to discard response bodies by default by switching the default `responseType` value to `none`. It can be enabled via the `--discard-response-bodies` CLI flag, the `K6_DISCARD_RESPONSE_BODIES` environment variable, or the `discardResponseBodies` script option:
```js
import http from 'k6/http';
export let options = {
  discardResponseBodies: true,
};
export default function () {
  let response = http.get("http://test.loadimpact.com", { responseType: "text" });
  // ... do something with the response, but ignore the contents of static files:
  http.batch([
    "http://test.loadimpact.com/images/logo.png",
    "http://test.loadimpact.com/style.css"
  ]);
};
```

Thanks to @sherrman for reporting the binary handling issues that prompted the addition of the `responseType` option! And thanks to @ofauchon for implementing both of the discard response body options, of which the local per-request one was later transformed into the `responseType=none` value!

## Internals

* Cloud output: improved outlier metric detection for small batches (#744)

## Bugs fixed!

* UI: The interactive `k6 login influxdb` command failed to write the supplied options to the config file. (#734)
* UI: Password input is now masked in `k6 login influxdb` and `k6 login cloud`. (#734)
* Config: Environment variables can now be used to modify k6's behavior in the `k6 login` subcommands. (#734)
* HTTP: Binary response bodies were mangled because there was no way to avoid converting them to UTF-16 JavaScript strings. (#749)
* Config: Stages were appended instead of overwritten from upper config "tiers", and were doubled when supplied via the CLI flag (#759)
* HTTP: `http.batch()` calls could panic because of a data race when the `batchPerHost` global option was used (#770)