## Why

Running a test once with 1 virtual user and 1 iteration is how people smoke-test, end-to-end test, and functionally test with k6. Today that needs either a per-environment edit to the script's scenario configuration, or a shortcut load flag. The shortcut flags replace every declared scenario with a generated `default` one, which drops `exec` targets, `env`, `tags`, and `options.browser`. A browser iteration can then fail with `browser not found in registry` while the process still exits 0, so CI and Synthetic Monitoring get no signal. Synthetic Monitoring needs a guaranteed single iteration on every check and has no runtime enforcement for it.

## What Changes

- Add a bare `--once` command-line flag to `k6 run` and `k6 cloud run`. It configures one effective `shared-iterations` scenario with 1 virtual user and 1 iteration, with no change to the script.
- Accept it only on those run commands, including `k6 cloud run --local-execution`, for script, archive, and standard-input sources they already support. Other commands reject it as unknown; `k6 archive` and `k6 cloud upload` are explicit regression cases.
- Preserve the running scenario's name, `exec` target, `options` block including `options.browser`, `env`, and `tags`.
- Discard every other scenario field, including its executor type and all load shaping. The scenario ends up as `shared-iterations` with `vus: 1`, `iterations: 1`, and the `shared-iterations` defaults for anything not preserved.
- Run the `default` function when the effective configuration has no scenarios. Run its one scenario when it has exactly one, including when that scenario targets `default`.
- Fail when the effective configuration has two or more scenarios, saying that single-run mode runs only a single scenario, without listing them.
- Fail when the effective configuration has no scenario and the script has no `default` function, exactly as k6 already fails for such a test.
- Reject `--once` combined with `--vus`, `--iterations`, `--duration`, or `--stage`, naming the conflicting flag.
- Keep single-run mode disabled by default and enable it only through bare `--once` on the current command line. A `once` script option, `K6_ONCE`, bare `ONCE`, or a `once` field in a configuration file or archive all leave it off. The flag has no scenario-selector form.
- Keep `setup()` and `teardown()` behaving exactly as they do without the flag.
- Carry the resulting 1 virtual user / 1 iteration scenario into the archives that `k6 cloud run` and `k6 cloud run --local-execution` upload. A later re-run also runs once when no other source supplies load.
- Reach 1 virtual user and 1 iteration for load declared by any non-CLI source: script options, `K6_VUS`/`K6_ITERATIONS`/`K6_DURATION`/`K6_STAGES`, the JSON configuration file, or an archive's stored options. The running scenario's non-load configuration survives. A load flag on the same command line is refused instead of overridden.
- Keep `thresholds` and every other non-scenario option untouched, so a threshold still fails the single run and sets the exit code.
- Rewrite scenario configuration after configuration consolidation, clear the four top-level load shortcuts, and leave every other option unchanged. Engine-level options such as `--paused` and `--execution-segment` keep their normal behavior, so they can delay or scale the resulting single-run scenario.

## Non-goals

- Naming a scenario to run, as `--once=<name>`. That is a separate change.
- Running every declared scenario once. That was considered and rejected, because a test with N scenarios would run N virtual users, defeating the goal.
- Changing what shortcut flags do on their own, without `--once`.
- Changing the end-of-test summary. A single run reports the same metrics as any other run.

## Capabilities

### New Capabilities

- once: A single-run mode, turned on by the bare `--once` command-line flag, that configures one scenario with 1 virtual user and 1 iteration while keeping its identity and non-load configuration.

### Modified Capabilities

None.

## Impact

- The flag sets of `run` and `cloud run`, including local execution and archive execution. The flag cannot be added to the option flag set that `run`, `archive`, `cloud run`, and `cloud upload` share, because two of those must reject it.
- Scenario and executor configuration, rewritten after consolidation.
- How k6 merges and derives execution options. `lib.Options.Apply` drops `scenarios` for a higher-tier `iterations`, `duration`, or `stages`; a lone `vus` can replace them during shortcut derivation. Neutralizing that load without losing the scenario reaches across this pipeline. This is the largest and riskiest part of the change, and it is deliberate: without it a stray environment variable silently defeats the flag.
- Archive generation for both cloud paths, which today deliberately store pre-derivation options.
- Browser test execution, which depends on the scenario's name and `options.browser` surviving into the run.
- Error and exit-code reporting for rejected invocations.
