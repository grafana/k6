# k6's WebCrypto API Web Platform Tests 

To be sure that we're compliant with the WebCrypto API specification, we run the [Web Platform Tests](https://web-platform-tests.org/) for the [WebCrypto API](https://www.w3.org/TR/WebCryptoAPI/) against our implementation. This is part of the CI process, and we expect to implement the missing tests when needed. 

Sometimes, we need to patch the tests to make them compatible with the k6 runtime, all current patches are available in the `wpt-patches` catalog. We try to keep the diff as small as possible.

The entry point is the [`checkout.sh`](./checkout.sh) script, which checks out the last commit sha of 
[wpt](https://github.com/web-platform-tests/wpt) that was tested with this module, and applies some patches
(all the `*.patch` files from the wpt-patches catalog) on top of it, in order to make the tests compatible with the k6 runtime.

If you work on a new web platform test, you could easily re-generate patches by running `./generate-patches.sh`.

**How to use**
1. Run `./checkout.sh` to check out the web-platform-tests sources.
2. Run `go test ../... -tags=wpt` to run the tests.
