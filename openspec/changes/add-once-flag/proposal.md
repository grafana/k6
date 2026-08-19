## Why

Running a script once with 1 virtual user and 1 iteration is how people smoke-test, end-to-end test, and functionally test with k6, but today that requires either editing the script's scenario configuration per environment or passing shortcut load flags that discard scenarios, ignore declared load, and break browser tests. Synthetic Monitoring needs this behavior for every check it runs and currently has no runtime enforcement at all, so k6 needs a built-in, guaranteed single-run mode.

## What Changes

- Add a `--once` command-line flag to `k6 run` and `k6 cloud run` that runs a script exactly once with 1 virtual user and 1 iteration, with no change to the script.
- Accept `--once` for `k6 cloud run --local-execution` and for running an already-built archive; the archive-building command does not accept the flag, and passing it there fails as an unknown flag with a non-zero exit code.
- Preserve the running scenario's exec target, its scenario options (browser options included), its environment variables, and its tags; discard its load-shaping fields (virtual users, iterations, duration, start time, maximum duration, time unit, stages, rate, graceful stop, graceful ramp-down, pre-allocated virtual users).
- Run exactly one iteration on exactly one virtual user, with no ramping, whatever executor the script declared; the executor becomes shared-iterations.
- Run the default function when the script declares no scenarios; run the one declared scenario when the script declares exactly one, including when that scenario targets the default function.
- Fail with a non-zero exit code when the script declares two or more scenarios, naming the available scenarios.
- Fail with a non-zero exit code when the script declares neither scenarios nor a default function, matching how k6 already behaves in that case.
- Reject `--once` combined with any command-line flag that shapes load (virtual users, iterations, duration, stages), with an error naming the conflicting flag and a non-zero exit code.
- Accept `--once` only as a command-line flag: neither an environment variable nor a configuration-file field turns single-run mode on, so a stray setting can never turn a real load test into a single run.
- Keep the script's setup and teardown steps running exactly as they do without the flag.
- Make declared load irrelevant whatever its source, whether it comes from script options, environment variables, or a configuration file: the run still ends at 1 virtual user and 1 iteration, while the scenario's non-load settings still apply.
- Carry the resulting 1 virtual user / 1 iteration scenario into generated cloud archives, so a later re-run of that archive also runs once.

## Capabilities

### New Capabilities

- once: A built-in single-run mode, turned on by the `--once` command-line flag, that runs a script exactly once with 1 virtual user and 1 iteration while preserving the scenario's configuration.

### Modified Capabilities

None.

## Impact

- The `run` and `cloud run` command-line surfaces, including local execution and archive execution, plus the archive command's flag set.
- The scenario and executor configuration layer, where single-run mode replaces load shaping.
- Cloud archive generation and later replay of those archives.
- Browser test execution, which depends on scenario-level browser options surviving into the single run.
- Error and exit-code reporting for rejected invocations.
