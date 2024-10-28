# k6's WebCrypto API Web Platform Tests 

This directory contains some utilities to run the [Web Platform Tests](https://web-platform-tests.org/) for the 
[WebCrypto API](https://www.w3.org/TR/WebCryptoAPI/) against the experimental module available in k6 as
`k6/x/webcrypto`.

The entry point is the [`checkout.sh`](./checkout.sh) script, which checks out the last commit sha of 
[wpt](https://github.com/web-platform-tests/wpt) that was tested with this module, and applies some patches
(all the `*.patch` files from the wpt-patches catalog) on top of it, in order to make the tests compatible with the k6 runtime.

If you work on a new web platfrom test, you could easily re-generate patches by running `./generate-patches.sh`.

We try to keep the diff as small as possible, and we aim to upstream the changes to the wpt repository.

**How to use**
1. Run `./checkout.sh` to check out the web-platform-tests sources.
2. Run `go test ../... -tags=wpt` to run the tests.
