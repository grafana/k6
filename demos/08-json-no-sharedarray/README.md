# JSON demo without SharedArray

Loads a generated 5-10 MB JSON file without `SharedArray` (per-iteration parse pattern).

## Files

- `script.js` (`100` VUs)
- `run.sh`
- `../generate_data.sh` (shared data generator)

## Run

```bash
./run.sh
```

`run.sh` generates `data.json` on demand before starting k6.
