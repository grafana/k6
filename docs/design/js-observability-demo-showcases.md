# JS Observability Demo Showcases

This document packages concrete, issue-driven demo segments for a short feature video
showing that JS profiling is actionable for real k6 user problems.

## Curated showcase cases

### Case 1: Init memory hotspot

- **User pain**: startup/init memory spikes, often from load/preprocess patterns.
- **Issue mapping**:
  - [Large init data memory pressure #5080](https://github.com/grafana/k6/issues/5080)
  - [SharedArray async limitation #3014](https://github.com/grafana/k6/issues/3014)
- **Primary signal**:
  - `scope=init`
  - top `alloc_space` file:line
  - first-runner memory milestones + top 5 lines
- **Demo script**: `examples/observability/js_observability_baseline.js`

### Case 2: VU runtime allocation churn

- **User pain**: memory drift or throughput degradation during steady-state execution.
- **Issue mapping**:
  - [High memory usage #3269](https://github.com/grafana/k6/issues/3269)
- **Primary signal**:
  - compare `scope=init` vs `scope=vu`
  - prove runtime code dominates allocations
- **Demo script**: `examples/observability/js_observability_baseline.js`

### Case 3: Async/browser attribution and isolation

- **User pain**: browser async operations are slow/heavy and hard to attribute.
- **Issue/forum mapping**:
  - [k6-browser networkidle timeout #5092](https://github.com/grafana/k6/issues/5092)
  - [xk6-browser hang behavior #1450](https://github.com/grafana/xk6-browser/issues/1450)
  - [Forum: k6-browser page.goto stops working](https://community.grafana.com/t/k6-browser-navigation-via-page-goto-stops-working/97306)
- **Primary signal**:
  - large alloc attribution in screenshot scenario
  - smaller scenario remains smaller in parallel run
  - async wait/run interpretation cue
- **Demo scripts**:
  - `examples/observability/browser_fullpage_alloc_repro.js`
  - `examples/observability/two_scenarios_alloc_repro.js`

### Case 4: REST-driven profiling automation

- **User pain**: teams need API-driven control and integration with external tooling.
- **Issue/forum mapping**:
  - [REST API: get total/current duration #1111](https://github.com/loadimpact/k6/issues/1111)
  - [Forum: Grafana k6 category](https://community.grafana.com/c/grafana-k6/70)
- **Primary signal**:
  - toggle profiling with REST
  - fetch JSON observability state and scoped artifacts
- **Endpoints**:
  - `GET /v1/js-observability`
  - `GET /v1/js-observability/profiling`
  - `POST /v1/js-observability/profiling/enable`
  - `POST /v1/js-observability/profiling/disable`
  - `GET /debug/pprof/js-cpu?scope=init|vu|combined`

## Recorder runbook (video order)

Use this fixed segment pattern:

1. state the user pain (linked issue/forum)
2. run one profiling command
3. show one decisive profiler output
4. state one remediation decision enabled by the output

### Segment 0 (opening, 30-45s)

- Explain `combined` for discovery, then split into `init` and `vu` for ownership.
- Mention output trio: CPU profile, runtime trace, first-runner memory top lines.

### Segment 1 (init memory hotspot, 45-60s)

```bash
k6 run \
  --address localhost:6565 \
  --js-profiling-enabled \
  --js-profiling-scope=init \
  --js-cpu-profile-output=demo-init.pprof \
  --js-runtime-trace-output=demo-init.trace \
  --js-first-runner-mem-max-bytes=500mb \
  --js-first-runner-mem-step-percent=5 \
  examples/observability/js_observability_baseline.js
```

Show:

- first-runner milestone logs
- top 5 alloc lines at end
- `go tool pprof demo-init.pprof` top lines

### Segment 2 (vu runtime churn, 45-60s)

```bash
k6 run \
  --js-profiling-enabled \
  --js-profiling-scope=vu \
  --js-cpu-profile-output=demo-vu.pprof \
  --js-runtime-trace-output=demo-vu.trace \
  examples/observability/js_observability_baseline.js
```

Show:

- top `alloc_space` and CPU lines from `demo-vu.pprof`
- side-by-side mention of Segment 1 vs Segment 2 winners

### Segment 3 (async/browser attribution, 60-90s)

```bash
k6 run \
  --js-profiling-enabled \
  --js-profiling-scope=vu \
  --js-cpu-profile-output=demo-browser-isolation.pprof \
  --js-runtime-trace-output=demo-browser-isolation.trace \
  examples/observability/two_scenarios_alloc_repro.js
```

Show:

- screenshot scenario has dominant allocation cost
- small allocation scenario stays materially smaller
- async wait/run interpretation note for browser operations

Optional single-scenario warm-up:

```bash
k6 run \
  --js-profiling-enabled \
  --js-profiling-scope=vu \
  --js-cpu-profile-output=demo-browser-single.pprof \
  --js-runtime-trace-output=demo-browser-single.trace \
  examples/observability/browser_fullpage_alloc_repro.js
```

### Segment 4 (REST automation, 45-60s)

Start a run with API enabled and then drive it:

```bash
curl -sS http://localhost:6565/v1/js-observability/profiling
curl -sS -X POST http://localhost:6565/v1/js-observability/profiling/enable
curl -sS http://localhost:6565/v1/js-observability
curl -sS http://localhost:6565/debug/pprof/js-cpu?scope=combined --output demo-rest-combined.pprof
curl -sS -X POST http://localhost:6565/v1/js-observability/profiling/disable
```

Show:

- JSON `profiling_enabled` state changes
- `first_runner_memory` block
- artifact availability and scoped profile fetch

## Pass-signal checklist (pre-recording validation)

### Segment 1 (init)

- `demo-init.pprof` exists and is readable by `go tool pprof`.
- top `alloc_space` lines resolve to JS file:line.
- first-runner milestone output is present.
- top 5 alloc lines are printed at end.

### Segment 2 (vu)

- `demo-vu.pprof` exists and is readable.
- VU hotspots differ from, or are more dominant than, init hotspots.
- `alloc_objects` and `alloc_space` sample types are present.

### Segment 3 (async/browser)

- browser run completes and emits screenshot size log.
- combined parallel scenario attributes large alloc to screenshot path.
- small scenario remains clearly lower than screenshot scenario.
- trace/profile show async-related execution around browser path.

### Segment 4 (REST)

- `GET /v1/js-observability` returns JSON with:
  - `profiling_enabled`
  - `first_runner_memory`
  - `artifacts_available`
- profiling enable/disable endpoints return successful state transitions.
- scoped CPU profile download returns binary pprof data.

## Narration snippets (30-45s each)

### Segment 1 narration

"This case mirrors init-memory complaints. I run init-only profiling, and immediately get
top alloc lines plus first-runner memory milestones. Instead of guessing, we can point to
the exact startup code path that allocates most bytes."

### Segment 2 narration

"Now I switch to VU-only scope. This separates runtime churn from startup cost. If VU
hotspots dominate, optimization belongs in iteration logic, not init preloading."

### Segment 3 narration

"Here we run two scenarios in parallel: a full-page browser screenshot and a small
allocation path. The profile shows the heavy browser alloc where it belongs and keeps
the small scenario small, which proves attribution isolation in practice."

### Segment 4 narration

"Finally, this is how teams automate profiling in CI or external systems. We toggle
profiling via REST, fetch JSON state, then pull scoped profiles for automated analysis."

## False-positive guardrails for async attribution

- Prefer relative comparisons (heavy scenario vs light scenario), not exact bytes.
- Validate with at least one repeat run to reduce sampling variance.
- Use both profile and trace signals before concluding attribution regression.
- If noise increases, narrow scenario concurrency for diagnosis runs.

## Cookbook alignment

This showcase pack follows the workflow and interpretation model in
`docs/design/js-observability-profiling-cookbook.md`:

- discovery in `combined`, ownership in `init`/`vu`
- symptom-driven triage (memory growth, browser async latency, runtime churn)
- REST-driven integration path for automation
