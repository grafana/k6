TODO: Intro

## New Features!

### HTTP: request body compression (#988)

Now all http methods have an additional param called `compression` that will make k6 compress the body before sending it. It will also correctly set both `Content-Encoding` and `Content-Length`, unless they were manually set in the request `headers` by the user. The current supported algorithms are `deflate` and `gzip` and any combination of the two separated by a comma (`,`).

## Bugs fixed!

* JS: Many fixes for `open()`: (#965)
  - don't panic with an empty filename (`""`)
  - don't make HTTP requests (#963)
  - correctly open simple filenames like `"file.json"` and paths such as `"relative/path/to.txt"` as relative (to the current working directory) paths; previously they had to start with a dot (i.e. `"./relative/path/to.txt"`) for that to happen
  - windows: work with paths starting with `/` or `\` as absolute from the current drive

* JS: Correctly always set `response.url` to be the URL that was ultimately fetched (i.e. after any potential redirects), even if there were non http errors. (#990)

## Internals

* JS: VU initialization time and memory usage has been significantly decreased by caching the parsed version of the core-js library. Thanks, @matlockx! (#1038)
* CI: removed the gometalinter check in CircleCI, since that project [was deprecated](https://github.com/alecthomas/gometalinter/issues/590) and now exclusively rely on [golangci-lint](https://github.com/golangci/golangci-lint). (#1039)