# k6/x/events example extension

A minimal k6 Go extension demonstrating every event type exposed by
`go.k6.io/k6/v2/x/events`: `Init`, `TestStart`, `IterStart`, `IterEnd`,
`TestEnd`, and `Exit`.

It subscribes to all six event types, logs each one as it fires (via k6's
logger), and tallies how many times each has fired. The tallies are exposed
to JS as `events.counts()` so `example.js` can assert every event type was
observed.

## Build

Requires [xk6](https://github.com/grafana/xk6). Because this example uses
`go.k6.io/k6/v2/x/events`, which may not yet be part of a released k6
version, build against this local checkout:

```sh
go install go.k6.io/xk6/cmd/xk6@latest

xk6 build \
  --replace go.k6.io/k6/v2=/absolute/path/to/this/k6/checkout \
  --with go.k6.io/k6/examples/extensions/events=/absolute/path/to/this/k6/checkout/examples/extensions/events \
  --output ./k6-with-events
```

(Once `x/events` ships in a released k6 version, the `--replace` override
can be dropped.)

## Run

```sh
./k6-with-events run examples/extensions/events/example.js
```

## Expected output

One log line per event, roughly in this order: `Init`, `TestStart`, then 6
`IterStart`/`IterEnd` pairs (each with `vuID`, `iteration`, `scenario`
fields) interleaved across VUs, then the `teardown()` console.log of counts,
then `TestEnd`, then `Exit` (with an `error` field, `<nil>` on a clean run).
