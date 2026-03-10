# Async browser attribution and isolation demo

Runs browser-related scenarios to show async allocation attribution and scenario isolation.

## Files

- `parallel.js`: two-scenario run (heavy screenshot + small alloc).
- `single.js`: optional single-scenario warm-up.
- `run.sh`: launcher for both modes.

## Run

Parallel two-scenario demo:

```bash
./run.sh
```

Optional single-scenario warm-up:

```bash
./run.sh single
```

## Outputs

- `out/demo-browser-isolation.pprof` and `out/demo-browser-isolation.trace` (parallel)
- `out/demo-browser-single.pprof` and `out/demo-browser-single.trace` (single)
