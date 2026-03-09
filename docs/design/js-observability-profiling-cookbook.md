# JS Profiling Cookbook

This playbook helps troubleshoot k6 scripts with JS-attributed CPU and allocation profiling.

## First triage

1. Run with JS profiling enabled and start with `combined` scope.
2. If needed, split into `init` and `vu` scopes to isolate startup vs runtime.
3. Pull `js-cpu` (or `js-cpu-init` / `js-cpu-vu`) and inspect:
   - `alloc_space`
   - `alloc_objects`
   - CPU hotspots by file and line
4. Check first-runner memory milestones and top alloc lines.

Interpretation quick guide:

- High `alloc_space`, low CPU: likely data movement/serialization churn.
- High CPU, moderate alloc: compute-heavy JS logic.
- Async wait much bigger than run: dominated by external operation latency.
- Async run much bigger than wait: dominated by callback/continuation JS work.

## Symptom-driven workflows

### Memory keeps climbing

Use `init` first, then `vu`.

1. Capture `init` profile and inspect top `alloc_space` file:line.
2. If init is not dominant, capture `vu`.
3. Compare top lines across scopes.

Typical fixes:

- Move expensive transforms into one-time init preprocessing.
- Avoid repeated large object cloning in iteration paths.
- Reduce repeated `JSON.stringify` / `JSON.parse` churn.
- Keep `SharedArray` usage read-only and avoid per-iteration materialization.

### Browser async hangs/timeouts

Use `vu` scope and async wait/run signals.

1. Capture `vu` profile and trace.
2. Compare wait vs run portions of async operations.
3. Confirm whether the issue is external wait or JS callback work.

Typical fixes:

- Replace brittle readiness checks with explicit assertions.
- Reduce heavy post-response parsing in callbacks.
- Narrow expensive browser operations (for example, full-page screenshots) to where needed.

### Startup is slow

Use `init` only.

1. Capture init profile.
2. Inspect top CPU and alloc file:line.
3. Review module-level code and large `open()`/parsing paths.

### Throughput is low in steady state

Use `vu` scope.

1. Capture `vu` profile.
2. Inspect hot default/setup/teardown paths.
3. Focus on reducing per-iteration allocation churn.

## First-runner memory milestone workflow

1. Configure first-runner max memory and step percent.
2. Run once and review milestone logs/report.
3. Use top file:line per milestone to prioritize fixes.
4. Review end-of-run top 5 file:line allocation summary.

Tip: start with coarse thresholds (5-10%), then tighten once a hotspot is identified.

## Scope strategy

- Use `combined` for discovery.
- Use `init` and `vu` to assign ownership precisely.
- Keep baseline outputs per scenario to compare regressions over time.

## REST automation pattern

Use REST endpoints to automate profiling windows and ingestion:

- Poll profiling state.
- Toggle profiling on/off around windows of interest.
- Fetch observability JSON and artifact availability.
- Pull profile artifacts and persist top offenders.

This is useful for CI regression checks, nightly soak snapshots, and threshold-triggered triage.

## CI guardrails

- Fail if a top `alloc_space` line in `vu` exceeds a budget.
- Warn if `init` allocation grows over baseline by a threshold.
- Alert if async wait/run ratio regresses for browser scenarios.

## Pitfalls

- Looking only at `combined` can hide whether the issue is init or runtime.
- Looking only at CPU can miss allocation-heavy regressions.
- Very short runs can under-sample; ensure enough activity window.

## Useful references

- [High memory usage #3269](https://github.com/grafana/k6/issues/3269)
- [SharedArray async limitation #3014](https://github.com/grafana/k6/issues/3014)
- [Large init data memory pressure #5080](https://github.com/grafana/k6/issues/5080)
- [k6-browser networkidle timeout #5092](https://github.com/grafana/k6/issues/5092)
- [xk6-browser hang behavior #1450](https://github.com/grafana/xk6-browser/issues/1450)
- [k6 REST API docs](https://grafana.com/docs/k6/latest/reference/k6-rest-api/)
- [Grafana k6 community forum](https://community.grafana.com/c/grafana-k6/70)
