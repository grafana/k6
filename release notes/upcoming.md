TODO: Intro

## New Features!

### CLI/Options: Add `--tag` flag and `tags` option to set test-wide tags (#553)

You can now specify any number of tags on the command line using the `--tag NAME=VALUE` flag. You can also use the `tags` option to the set tags in the code.

The specified tags will be applied across all metrics. However if you have set a tag with the same name on a request, check or custom metric in the code that tag value will have precedence.

Thanks @antekresic for their work on this!

**Docs**: [Test wide tags](https://docs.k6.io/v1.0/docs/tags-and-groups#section-test-wide-tags) and [Options](https://docs.k6.io/v1.0/docs/options#section-available-options)

## Bugs fixed!

* Category: description of bug. (#PR)

## UX

* Clearer error message when using `open` function outside init context (#563)