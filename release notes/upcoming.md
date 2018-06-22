TODO: Intro

## New Features!
* New JS API to set seed for PRNG. Now, you are able to set a seed to get reproducible (pseudo-)random numbers. (#677)

```js
import {randomSeed} from "k6";

randomSeed(123456789);
let rnd = Math.random();
console.log(rnd)
```

* New option `--no-vu-connection-reuse` that let's users close the connections between iterations of a VU. (#676)

### Category: Title (#533)

Description of feature.

**Docs**: [Title](http://k6.readme.io/docs/TODO)


## UX

* New option to reset cloud token (#672)


## Internals

### Real-time metrics (#678)

Previously most metrics were emitted only when a script iteration ended. With these changes, metrics would be continuously pushed in real-time, even in the middle of a script iteration. This should slightly decrease memory usage and help a lot with the aggregation efficiency of the cloud collector.


## Bugs fixed!

* Metrics emitted by `setup()` and `teardown()` are not discarded anymore. They are emitted and have the implicit root `group` tag values of `setup` and `teardown` respectively (#678)


## Breaking Changes
* The `--no-connection-reuse` option has been re-purposed and now disables keep-alive connections globally. The newly added `--no-vu-connection-reuse` option does what was previously done by `--no-connection-reuse` - it closes any open connections between iterations of a VU, but allows for reusing them inside of a single iteration. (#676)
