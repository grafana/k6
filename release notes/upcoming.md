TODO: Intro

## New Features!

### HTTP: request body compression (#988)

Now all http methods have an additional param called `compression` that will make k6 to compress the body before sending it. It will also correctly set both `Content-Encoding` and `Content-Length`, except if they were not set previous to that. The current supported algorithms are `deflate` and `gzip` and any combination of the two separated by a comma (`,`).

## Bugs fixed!

* JS: Many fixes for `open()`: (#965)
  - don't panic with an empty filename (`""`)
  - don't make HTTP requests (#963)
  - correctly open simple filenames like `"file.json"` and paths such as `"relative/path/to.txt"` as relative (to the current working directory) paths; previously they had to start with a dot (i.e. `"./relative/path/to.txt"`) for that to happen
  - windows: work with paths starting with `/` or `\` as absolute from the current drive
