# Streams API Web Platform Tests 

This directory contains some utilities to run the [Web Platform Tests](https://web-platform-tests.org/) for the 
[Streams API](https://streams.spec.whatwg.org/) against the experimental module available in k6 as
`k6/experimental/streams`.

The entry point is the [`checkout.sh`](./checkout.sh) script, which checks out the last commit sha of 
[wpt](https://github.com/web-platform-tests/wpt) that was tested with this module, and applies some patches
(all the `*.patch` files) on top of it, in order to make the tests compatible with the k6 runtime.

**How to use**
1. Run `./checkout.sh` to check out the web-platform-tests sources.
2. Run `go test ../... -tags=wpt` to run the tests.
