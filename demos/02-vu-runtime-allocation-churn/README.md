# VU runtime allocation churn demo

Runs VU-scoped profiling to isolate runtime allocation churn from init-time costs.

## Files

- `script.js`: self-contained k6 workload script for this demo.
- `run.sh`: run command with VU-focused profiling flags.

## Run

```bash
./run.sh
```

## Outputs

- `out/demo-vu.pprof`
- `out/demo-vu.trace`
