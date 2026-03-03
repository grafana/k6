# JS Observability Baseline

This baseline script is used to compare overhead and fidelity before/after changes
in JS-attributed profiling and runtime trace capture.

## Script

- `scripts/observability/js_observability_baseline.js`

The script intentionally has:

- short-lived allocations in `allocateBurst()`
- CPU-heavy math in `cpuBurst()`
- Promise/`await` callback chains in `asyncBurst()`
- steady pacing via `sleep(0.1)`

## Suggested Runs

### Baseline (no JS observability)

```bash
k6 run scripts/observability/js_observability_baseline.js
```

### With JS observability

```bash
k6 run \
  --js-profiling-enabled \
  --js-cpu-profile-output=baseline-js.pprof \
  --js-runtime-trace-output=baseline-js.trace \
  scripts/observability/js_observability_baseline.js
```

## Validation Artifacts

- CPU profile: `baseline-js.pprof`
- runtime trace: `baseline-js.trace`
- optional API retrieval when profiling API is enabled:
  - `/debug/pprof/js-cpu`
  - `/debug/pprof/js-trace`

## Async-specific checks

- In `go tool pprof baseline-js.pprof`, verify `alloc_objects` and `alloc_space` sample
  types are present.
- Verify async samples include labels such as `js.async.op_id`, `js.async.parent_op_id`,
  and `js.async.wait_ns`.
- In `go tool trace baseline-js.trace`, inspect `sobek.async.promise_reaction` regions and
  confirm callbacks are visible under JS run phases.
