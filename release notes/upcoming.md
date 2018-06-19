TODO: Intro

## New Features!

* New option `--no-vu-connection-reuse` that let's users close the connections between iterations of a VU. (#676)

### Category: Title (#533)

Description of feature.

**Docs**: [Title](http://k6.readme.io/docs/TODO)

## UX
* New option to reset cloud token (#672)

## Bugs fixed!

* Category: description of bug. (#PR)

## Breaking Changes
* The `--no-connection-reuse` option has been re-purposed and now disables keep-alive connections globally. The newly added `--no-vu-connection-reuse` option does what was previously done by `--no-connection-reuse` - it closes any open connections between iterations of a VU, but allows for reusing them inside of a single iteration. (#676)
