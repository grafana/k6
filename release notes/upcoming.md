TODO: Intro

## New Features!
* New JS API to set seed for PRNG. Now, you are able to set a seed to get reproducible (pseudo-)random numbers. (#677)

```js
import {randomSeed} from "k6";

randomSeed(123456789);
let rnd = Math.random();
console.log(rnd)
```

* A new option `--no-vu-connection-reuse` lets users close HTTP `keep-alive` connections between iterations of a VU. (#676)


## UX

* There's a new option to reset the saved cloud token: `k6 login cloud --reset` (#672)
* The check and group names in the summary at the end of a test now appear in the order they were defined. Thanks to @mohanprasaths for fixing this! (#674)

## Internals

### Real-time metrics (#678)

Previously most metrics were emitted only when a script iteration ended. With these changes, metrics would be continuously pushed in real-time, even in the middle of a script iteration. This should slightly decrease memory usage and help a lot with the aggregation efficiency of the cloud collector.

### Automated deb, rpm, msi and nuget package builds (#675)

We now automatically build packages for different operating systems and upload them to bintray on every new release: [https://bintray.com/loadimpact](https://bintray.com/loadimpact/)

TODO: information about how to add the bintray repos and install k6


## Bugs fixed!

* Metrics emitted by `setup()` and `teardown()` are not discarded anymore. They are emitted and have the implicit root `group` tag values of `setup` and `teardown` respectively (#678)
* Fixed a potential `nil` pointer error when the `k6 cloud` command is interrupted. (#682)

## Breaking Changes
* The `--no-connection-reuse` option has been re-purposed and now disables keep-alive connections globally. The newly added `--no-vu-connection-reuse` option does what was previously done by `--no-connection-reuse` - it closes any open connections between iterations of a VU, but allows for reusing them inside of a single iteration. (#676)
