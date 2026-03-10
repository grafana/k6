# REST-driven profiling automation demo

Starts a k6 run with API enabled, toggles profiling over REST, fetches observability JSON, and downloads a scoped CPU profile.

## Files

- `script.js`: self-contained k6 workload script run in the background.
- `run.sh`: REST workflow automation script.

## Run

```bash
./run.sh
```

## Outputs

- `out/profiling-before.json`
- `out/profiling-enable.json`
- `out/js-observability.json`
- `out/demo-rest-combined.pprof`
- `out/profiling-disable.json`
- `out/k6.log`
