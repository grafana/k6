# Init memory hotspot demo

Runs init-scoped JS profiling to surface startup memory hotspots and first-runner memory milestones.

## Files

- `script.js`: self-contained k6 workload script for this demo.
- `run.sh`: run command with init-focused profiling flags.

## Run

```bash
./run.sh
```

## Outputs

- `out/demo-init.pprof`
- `out/demo-init.trace`
