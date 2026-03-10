# CSV demo with experimental/csv

Loads a generated 5-10 MB CSV file and parses it with `k6/experimental/csv`.

## Files

- `script.js` (`100` VUs)
- `run.sh`
- `../generate_data.sh` (shared data generator)

## Run

```bash
./run.sh
```

`run.sh` generates `data.csv` on demand before starting k6.
