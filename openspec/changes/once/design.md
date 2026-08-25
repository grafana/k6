## Context

`loadedTest.consolidateDeriveAndValidateConfig()` initializes the runner, merges file, runner,
environment, and CLI options in `getConsolidatedConfig()`, then calls `deriveAndValidateConfig()`.
Load shortcuts at the top level can remove a scenario during `Config.Apply()` or replace it in
`executor.DeriveScenariosFromShortcuts()`.

Cloud execution adds two archive boundaries. `runCloudTest()` resets the runner to consolidated
options before creating the remote archive. `createCloudTest()` creates the local execution archive
before `cmdRun.run()` reinjects derived options.

## Goals / Out of scope

**Goals:**

- Keep `--once` command scoped while producing one validated `shared-iterations` scenario for local
  and cloud execution.
- Preserve the selected scenario through configuration precedence and put the same transformed
  execution settings in cloud archives and provisioning requests.
- Cover the transformation at the narrowest unit, command, and archive test boundaries already
  present in the repository.

**Out of scope:**

- Add a `once` option to scripts, environment variables, JSON configuration, or archives.
- Change shortcut precedence without `--once`, add scenario selection, or expose the flag on
  `archive` or `cloud upload`.

## Decisions

### Keep flag state on the run commands

Register `--once` in `cmdRun.flagSet()` and `cmdCloudRun.flagSet()`, and pass its command state into
the load and configuration flow. Keep it out of `optionFlagSet()`, `Config`, and `lib.Options`. A
shared command check rejects conflicting load flags. During consolidation, reject load environment
variables `K6_VUS`, `K6_ITERATIONS`, `K6_DURATION`, and `K6_STAGES` after they are parsed and before
they are applied. Both checks run before a VU function or cloud request starts. Adding the flag to
the shared option model was rejected because that would expose configuration and serialization
paths and would make `archive` and `cloud upload` accept it.

### Protect scenarios while applying configuration layers

When the mode is active, `getConsolidatedConfig()` or a helper called from it must clear `vus`,
`iterations`, `duration`, and `stages` at the top level of script, JSON config, and archive option
layers before `Config.Apply()` can remove an existing scenario. CLI load flags have already failed
the command check. A combination of `--once` and load environment variables must fail after
`readEnvConfig()` parses them and before `Config.Apply()` applies them. Scenario maps still follow
the current precedence.

Clearing the fields only after consolidation was rejected because `Options.Apply()` may already
have discarded the scenario that must be preserved.

### Replace the effective scenario before derivation and validation

After runner init, consolidation, and defaulting, reject more than one effective scenario. Replace
zero or one scenario in `consolidatedConfig.Options` with a fresh
`executor.SharedIterationsConfig`. Preserve the map key, raw nullable `exec`, env, tags, and
scenario options, then set valid values of 1 for VUs and iterations. Leave `startTime`,
`maxDuration`, and `gracefulStop` with `Valid == false`. Keep the constructor defaults for
`maxDuration` and
`gracefulStop` so they serialize as null but run with the 10 minute and 30 second durations. Do not
use `ExecutorConfig.GetExec()` to copy `exec`, because it normalizes an unset value to `default`.
Clear the four load fields at the top level, then pass the result to `deriveAndValidateConfig()`.

This order lets initial decoding reject malformed values and unknown executors, lets the replacement
discard invalid load combinations, and lets normal validation check the scenario and exported
function that will run. Transforming after shortcut derivation was rejected because derivation may
already have replaced the selected scenario.

### Preserve execution segment allocation

Execution segments divide one logical test among k6 processes. They do not define the test's total
load. Apply the existing execution segment allocation after `--once` creates the global 1 VU / 1
iteration scenario. Across a complete segment sequence, one segment receives the VU and its
iteration, and the remaining segments receive no work. Giving every segment a VU and iteration
would run the logical test more than once.

### Use transformed consolidated options for archives and derived options for execution

The remote path in `runCloudTest()` should keep archiving `test.consolidatedConfig.Options`; those
options now contain the replacement scenario when `--once` is active and retain existing behavior
otherwise. The local path must set the runner to the transformed consolidated options before
`createCloudTest()` calls `makeArchive()`, because archive creation currently precedes the
reinjection in `cmdRun.run()`.

The scheduler, local provisioning `options`, execution plan, `max_vus`, and `total_duration`
continue to use `derivedConfig.Options`. `--no-archive-upload` skips only archive creation and still
provisions from the derived replacement. Using derived options for every archive was rejected
because it would change how ordinary shortcut configurations are serialized. Only remote
`k6 cloud run` calls `validate_options`; local cloud execution is checked through its provisioning
request.

### Put tests at the existing boundaries

Keep source precedence and transformation tables beside `internal/cmd/config_consolidation_test.go`
and `internal/cmd/config_test.go`. Put local and cloud behavior in
`internal/cmd/tests/cmd_run_test.go` and `internal/cmd/tests/cmd_cloud_run_test.go`. Use the
existing v6 and local provisioning test servers to inspect requests and archives. Use one full
contract case per execution mode and smoke cases for the remaining file, archive, and stdin
transports.

Include nested browser options in the scenario transformation table with the other fields that the
transformation preserves.

## Risks and tradeoffs

- Clearing a load field too late can lose the selected scenario. Clear it on each layer before
  `Config.Apply()` and keep source matrix tests with and without `--once`.
- The runner, uploaded archive, and provisioning request can diverge. Keep transformed consolidated
  and derived options explicit, and compare captured archive and request data in both cloud paths.
