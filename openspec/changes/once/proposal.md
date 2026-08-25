## Why

People use a run with 1 virtual user and 1 iteration to smoke test and functionally test with k6. Today they must edit the script for each environment or use a shortcut load flag. Shortcut flags replace every declared scenario with a generated `default` scenario, which drops `exec`, `env`, tags, and `options.browser`. A browser iteration can then fail with `browser not found in registry` while the process exits with code 0, so CI and Synthetic Monitoring get no failure signal. Synthetic Monitoring needs to guarantee one iteration for every check without changing the script.

## What Changes

- Add bare `--once` to `k6 run`, `k6 cloud run`, and `k6 cloud run --local-execution`. It works with script files, archives, and standard input.
- Preserve the running scenario's name, `exec` target, `options` block including `options.browser`, `env`, and `tags`.
- Replace every other scenario field. The result uses `shared-iterations` with `vus: 1`, `iterations: 1`, and the defaults for that executor.
- Run the single effective scenario. If there is no scenario, create one named `default` and run the exported `default` function.
- Fail when the effective configuration has two or more scenarios.
- Fail when the effective configuration has no scenario and the script has no `default` function.
- Reject `--once` combined with `--vus`, `--iterations`, `--duration`, or `--stage`, naming the conflicting flag.
- Reject `--once` combined with `K6_VUS`, `K6_ITERATIONS`, `K6_DURATION`, or `K6_STAGES`, naming the conflicting variable.
- Enable the behavior only through bare `--once` on the current command line. A `once` script option, `K6_ONCE`, `ONCE`, or a `once` field in a JSON config or archive leaves it off. The flag does not select a scenario.
- Preserve setup, teardown, thresholds, pauses, and execution segments. A threshold can still fail the run, a pause can delay it, and execution segments divide the single VU and iteration across their full sequence. One segment receives the work; the others receive none.
- Carry the resulting 1 virtual user / 1 iteration scenario into the archives that `k6 cloud run` and `k6 cloud run --local-execution` upload. A later run of that archive also runs once when no other source supplies load.
- Ignore `vus`, `iterations`, `duration`, and `stages` from the script, JSON config, or archive without losing the selected scenario.
- Keep `k6 archive --once` and `k6 cloud upload --once` invalid.

## Out of scope

- Naming a scenario to run with `--once=<name>`.
- Running every declared scenario once. That was considered and rejected, because a test with N scenarios would run N virtual users, defeating the goal.
- Changing what shortcut flags do on their own, without `--once`.
- Changing the summary printed after the test. A run with `--once` reports the same metrics as any other run.

## Capabilities

### New Capabilities

- once: Bare `--once` configures one scenario with 1 virtual user and 1 iteration while keeping its identity and options unrelated to load.

### Modified Capabilities

None.

## Impact

- The flag sets for `run` and `cloud run`, including local and archive execution. The shared option flag set also serves `archive` and `cloud upload`, which must reject `--once`.
- Scenario and executor configuration after k6 combines all option sources.
- Configuration merging and shortcut derivation. Load shortcuts can remove scenarios before `--once` transforms them, so k6 must preserve the selected scenario while the flag is active.
- Archive generation for both cloud paths, which stores options before executor derivation.
- Browser scenarios, which depend on the scenario's name and `options.browser` surviving into the run.
- Errors and exit codes for rejected invocations.
