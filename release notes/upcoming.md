TODO: Intro

## New Features!

### Category: Title (#533)

Description of feature.

**Docs**: [Title](http://k6.readme.io/docs/TODO)

## Internals

### Real-time metrics (#678)

Previously most metrics were emitted only when a script iteration ended. With these changes, metrics would be continuously pushed in real-time, even in the middle of a script iteration. This should slightly decrease memory usage and help a lot with the aggregation efficiency of the cloud collector.

## Bugs fixed!

* Metrics emitted by `setup()` and `teardown()` are not discarded anymore. They are emitted and have the implicit root `group` tag values of `setup` and `teardown` respectively (#678)