## Why

Running a test once with 1 virtual user and 1 iteration is how people smoke-test, end-to-end test, and functionally test with k6. Today that needs either a per-environment edit to the script's scenario configuration, or a shortcut load flag. The shortcut flags replace every declared scenario with a generated `default` one, which drops `exec` targets, `env`, `tags`, and `options.browser`. Browser tests then fail with `browser not found in registry` and still exit 0, so CI and Synthetic Monitoring get no signal. Synthetic Monitoring needs a guaranteed single iteration on every check and has no runtime enforcement for it at all.

## What Changes

- Add a `--once` command-line flag to `k6 run` and `k6 cloud run` that runs a test exactly once with 1 virtual user and 1 iteration, with no change to the script.
- Accept it for `k6 cloud run --local-execution` and for running an already-built archive. Reject it on `k6 archive` and `k6 cloud upload`, the two commands that build an archive without running it, where it fails as an unknown flag with a non-zero exit code.
- Preserve the running scenario's name, `exec` target, `options` block including `options.browser`, `env`, and `tags`.
- Discard the running scenario's load shaping: `executor`, `vus`, `iterations`, `duration`, `startTime`, `maxDuration`, `timeUnit`, `stages`, `rate`, `gracefulStop`, `gracefulRampDown`, `preAllocatedVUs`. The scenario ends up as `shared-iterations` with `vus: 1` and `iterations: 1`, carrying that executor's defaults for anything not preserved.
- Run the `default` function when the test declares no scenarios. Run the one declared scenario when it declares exactly one, including when that scenario targets `default`.
- Fail when the test declares two or more scenarios, saying that single-run mode runs only a single scenario, without listing them.
- Fail when the test declares neither scenarios nor a `default` function, exactly as k6 already fails for such a test.
- Reject `--once` combined with `--vus`, `--iterations`, `--duration`, or `--stage`, naming the conflicting flag.
- Accept `--once` only from the command line: `K6_ONCE`, a bare `ONCE`, and a `once` configuration-file field all leave single-run mode off, so a stray setting can never turn a real load test into a single run. `--once` takes no value; `--once=false` leaves single-run mode off.
- Keep `setup()` and `teardown()` behaving exactly as they do without the flag.
- Carry the resulting 1 virtual user / 1 iteration scenario into the archives that `k6 cloud run` and `k6 cloud run --local-execution` upload, and keep those archives runnable, so a later re-run also runs once.
- Reach 1 virtual user and 1 iteration whatever source declared the load, with no exception: script options, `K6_VUS`/`K6_ITERATIONS`/`K6_DURATION`/`K6_STAGES`, the JSON configuration file, or an archive's stored options. The running scenario's non-load configuration survives that. Only a load flag on the same command line is refused outright instead of overridden.
- Keep `thresholds` and every other non-scenario option untouched, so a threshold still fails the single run and sets the exit code.
- Rewrite scenario configuration after configuration consolidation and change nothing else, so engine-level options such as `--paused` and `--execution-segment` keep their normal behavior.

## Non-goals

- Naming a scenario to run, as `--once=<name>`. That is a separate change.
- Running every declared scenario once. That was considered and rejected, because a test with N scenarios would run N virtual users, defeating the goal.
- Changing what shortcut flags do on their own, without `--once`.
- Changing the end-of-test summary. A single run reports the same metrics as any other run.

## Capabilities

### New Capabilities

- once: A single-run mode, turned on by the `--once` command-line flag, that runs a test exactly once with 1 virtual user and 1 iteration while keeping the running scenario's identity and non-load configuration.

### Modified Capabilities

None.

## Impact

- The flag sets of `run` and `cloud run`, including local execution and archive execution. The flag cannot be added to the option flag set that `run`, `archive`, `cloud run`, and `cloud upload` share, because two of those must reject it.
- Scenario and executor configuration, rewritten after consolidation.
- How k6 merges configuration sources. `lib.Options.Apply` drops the whole `scenarios` map whenever a tier sets `vus`, `iterations`, `duration`, or `stages`, so neutralizing declared load without losing the scenario reaches into the merge every tier passes through. This is the largest and riskiest part of the change, and it is deliberate: without it a stray environment variable silently defeats the flag.
- Archive generation for both cloud paths, which today deliberately store pre-derivation options.
- Browser test execution, which depends on the scenario's name and `options.browser` surviving into the run.
- Error and exit-code reporting for rejected invocations.
