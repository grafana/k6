After every task, format the changed files, run `make lint`, and run the tests affected by that
task. Run Go tests with `-race`. Fix failures before starting the next task.

## 1. Add the flag to run commands

- [ ] 1.1 Register `--once` as a command switch on the dedicated `run` and `cloud run` flag sets,
  carry its state through the command, and give it a clear help description. Do not add it to
  `Config`, `lib.Options`, or `optionFlagSet()`. Check: help tests show the flag on `k6 run` and
  `k6 cloud run` only, while `k6 archive --once` and `k6 cloud upload --once` fail as unknown flags.
- [ ] 1.2 Reject an active `--once` combined with a changed `--vus`, `--iterations`, `--duration`,
  or `--stage` flag, and report both `--once` and the conflicting long name. Check: one table covers
  every combination of the long and short spellings and the local, remote cloud, and local cloud
  run paths, including zero values. No VU function or cloud request starts.
- [ ] 1.3 After environment options are parsed, reject `K6_VUS`, `K6_ITERATIONS`, `K6_DURATION`,
  and `K6_STAGES` with `--once` before their load values can replace a scenario. Report `--once` and
  the conflicting variable. Check: one table covers every combination of the four variables and
  the local, remote cloud, and local cloud run paths, including a zero value. No VU function or
  cloud request starts.
- [ ] 1.4 Keep activation tied to bare `--once` on the current command. Check: a default run retains
  the declared load, `--once=api` fails parsing, and script options, `K6_ONCE`, `ONCE`, explicit and
  default JSON config, and archive metadata cannot enable the mode.

## 2. Build and validate the effective scenario

- [ ] 2.1 When `--once` is active, neutralize `vus`, `iterations`, `duration`, and `stages` at the
  top level of script, JSON config, and archive options before they can erase or replace the
  effective scenario. Leave all other options and normal precedence for the complete scenario map
  unchanged. Check: a source matrix preserves the script scenario when both the script and JSON
  config declare scenarios, preserves config and archive scenarios otherwise, and proves the
  existing shortcut replacement behavior is unchanged without `--once`.
- [ ] 2.2 Implement a focused transformation from zero or one effective scenario to a fresh
  `shared-iterations` scenario with `vus: 1` and `iterations: 1`. Preserve only its name, raw `exec`
  value, `env`, tags, and complete `options` block; clear the four load fields at the top level and
  discard every other original executor field. Check: one table test covers no scenario and all six
  current executor types. It verifies every preserved, set, and discarded field, including nested
  browser options; distinguishes missing `exec` from explicit `exec: default`; verifies JSON nulls
  for `startTime`, `maxDuration`, and `gracefulStop`; and verifies the 10 minute and 30 second
  runtime defaults.
- [ ] 2.3 Apply the transformation after init and consolidation for `--once`, but before shortcut
  derivation and final validation, in the configuration flow shared by local and cloud runs.
  Validate the configuration that will execute. Check: a discarded `vus > iterations` combination
  succeeds, while unparseable typed fields, an unknown executor, and missing default or named
  exports still fail before a VU function runs.
- [ ] 2.4 Reject two or more effective scenarios with an error about `--once` after init and before
  setup, VU initialization, browser launch, or a cloud request. Check: command tests exercise script
  and archive input through local, remote cloud, and local cloud paths, and observe the init marker
  but none of the later side effects.

## 3. Verify local execution

- [ ] 3.1 Add one full local command test with a script file, plus smoke cases for an archive file,
  script stdin, and archive stdin. Check: the full case keeps the declared scenario, selected
  function, env, tags, and options. Its JSON sample keeps the scenario name and custom tags. Each
  smoke case accepts its transport, prints the selected marker once, and completes one iteration.
- [ ] 3.2 Cover default function selection when scenarios are absent, empty, or null and when the
  script contains only VU, duration, iteration, or stage shortcuts at the top level. Check: every
  case creates a scenario named `default`, prints its marker once, and exposes no residual load
  value in the effective options.
- [ ] 3.3 Extend command integration tests for behavior unrelated to load that `--once` preserves.
  Check: setup data reaches the selected function and teardown in order; CLI and JSON skip options
  still work; a failing threshold exits 99; a paused run waits for REST resume; and the two
  execution segment halves allocate one VU and one iteration in total, with only one half receiving
  work.

## 4. Carry the scenario through cloud runs

- [ ] 4.1 Make archive creation use the transformed consolidated options in both cloud paths, while
  local provisioning and execution use the derived options. Check: focused tests verify the options
  passed at each boundary with `--once`, and prove behavior is unchanged without the flag.
- [ ] 4.2 Add one full script file contract test for each cloud mode. Check: both archives contain
  exactly one `shared-iterations` scenario with 1 VU and 1 iteration and preserve its name, `exec`,
  env, tags, and options. They serialize the four load fields at the top level plus scenario
  `startTime`, `maxDuration`, and `gracefulStop` as null and omit discarded executor fields and any
  `once` key. Only remote `k6 cloud run` sends `validate_options`, and its payload matches
  `metadata.json`; local provisioning sends `max_vus: 1` and `total_duration: 630`. Replaying either
  archive with plain `k6 run` runs once with the 10 minute maximum and 30 second graceful stop.
- [ ] 4.3 Smoke test archive file, script stdin, and archive stdin transport for both cloud modes.
  Check: each input uploads or provisions successfully, and each local case runs one iteration. The
  local `--no-archive-upload` case sends `archive_size: null`, performs no upload, and still runs
  once.

## 5. Verify the CLI manually

- [ ] 5.1 Build `./k6` and use temporary scripts and archives to exercise `k6 run --once` manually.
  Cover API and browser scenarios, a test without scenarios, load shortcuts, preserved scenario
  fields, rejected load conflicts, multiple scenarios, execution segments, and file, archive, and
  standard input. Check the command output, exit code, markers, and iteration count for each case.
- [ ] 5.2 Run `k6 cloud run --once` and `k6 cloud run --local-execution --once` against an
  authenticated Grafana Cloud test environment with representative API and browser scenarios.
  Record each test run ID and use `gcx` to inspect its result and metrics. Check that each successful
  cloud test ran one VU and one iteration. Inspect each uploaded archive and check its
  `metadata.json` contains the transformed scenario and preserved fields. If the archive cannot be
  retrieved, add temporary diagnostic output at the archive upload boundary, rebuild, and repeat
  the cloud runs. Remove all diagnostic code, temporary scripts, and archives after verification.
