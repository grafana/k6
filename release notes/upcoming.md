TODO: Intro

## New Features!

### Category: Title (#533)

Description of feature.

**Docs**: [Title](http://k6.readme.io/docs/TODO)

## Bugs fixed!

* JS: Many fixes for `open()`: (#965)
  - don't panic with an empty filename (`""`)
  - don't make HTTP requests (#963)
  - correctly open simple filenames like `"file.json"` and paths such as `"relative/path/to.txt"` as relative (to the current working directory) paths; previously they had to start with a dot (i.e. `"./relative/path/to.txt"`) for that to happen
  - windows: work with paths starting with `/` or `\` as absolute from the current drive
