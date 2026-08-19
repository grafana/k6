## ADDED Requirements

### Requirement: Single-run execution flag

The system MUST provide a `--once` command-line flag that runs a script exactly one time with 1 virtual user and 1 iteration, without requiring any edit to the script. The run MUST perform exactly one iteration on exactly one virtual user, with no ramping, whatever executor the script declared, and the load declared in the script MUST NOT take effect. The scenario's executor MUST become shared-iterations. The system MUST NOT run more than one scenario in a single invocation under any circumstance. The flag MUST be accepted by `k6 run`, by `k6 cloud run`, by `k6 cloud run --local-execution`, and when running an already-built archive, and it MUST produce the same single-run behavior in each. The archive-building command MUST NOT accept the flag.

#### Scenario: Identical behavior across every accepted entry point
- **GIVEN** a script whose single scenario declares 100 virtual users and 100 iterations under a ramping executor
- **WHEN** the user runs it with `--once` under `k6 run`
- **AND** separately under `k6 cloud run`
- **AND** separately under `k6 cloud run --local-execution`
- **AND** separately by running an already-built archive of that script with `--once`
- **THEN** each invocation runs exactly one iteration on exactly one virtual user
- **AND** each invocation uses the shared-iterations executor for that scenario
- **AND** none of the declared load is applied

#### Scenario: Archive building rejects the flag
- **GIVEN** any script
- **WHEN** the user passes `--once` to the archive-building command
- **THEN** k6 reports an unknown flag error
- **AND** the process exits with a non-zero code

### Requirement: Scenario configuration is preserved and load shaping is discarded

Single-run mode MUST preserve the running scenario's exec target, its scenario options including browser options, its environment variables, and its tags. Single-run mode MUST discard the scenario's load-shaping configuration: virtual users, iterations, duration, start time, maximum duration, time unit, stages, rate, graceful stop, graceful ramp-down, and pre-allocated virtual users.

#### Scenario: Non-load configuration survives, load configuration does not
- **GIVEN** a script with one scenario named `api` that sets an exec target, browser options, scenario environment variables, and tags
- **AND** whose load is arrival-rate shaped, declaring a rate, a time unit, pre-allocated virtual users, a start time, and a maximum duration
- **WHEN** the user runs it with `--once`
- **THEN** the run uses that scenario's exec target, browser options, environment variables, and tags
- **AND** exactly one iteration runs on exactly one virtual user
- **AND** the rate, time unit, pre-allocated virtual users, start time, and maximum duration have no effect on the run

### Requirement: Declared load never takes effect, whatever its source

Under single-run mode, the scenario's load-shaping configuration MUST have no effect regardless of which configuration source it reaches the scenario from: script options, an environment variable, or a configuration file. The scenario's non-load configuration MUST still apply from whichever of those sources it reaches the scenario from.

#### Scenario: Load supplied outside the script still runs once
- **GIVEN** a script whose scenario takes its virtual user and iteration counts from an environment variable
- **AND** whose scenario takes its tags from a configuration file
- **WHEN** the user runs it with `--once`
- **THEN** exactly one iteration runs on exactly one virtual user
- **AND** the scenario's tags from the configuration file still apply

### Requirement: Scripts without scenarios run the default function

When a script declares no scenarios and exports a default function, single-run mode MUST run that default function once with 1 virtual user and 1 iteration, and any load the script declares at the top level MUST NOT take effect.

#### Scenario: Default-only script runs once
- **GIVEN** a script with no scenarios that exports a default function
- **AND** declares 50 virtual users and a 30 second duration at the top level
- **WHEN** the user runs it with `--once`
- **THEN** the default function runs exactly one time on one virtual user
- **AND** the declared virtual users and duration have no effect

### Requirement: A single declared scenario is the one that runs

When a script declares exactly one scenario, single-run mode MUST run that scenario once, using its configuration. If the scenario names an exec target, that target MUST run even when the script also exports a default function, and the default function MUST NOT run. If the scenario names no exec target, or names the default function as its target, the default function MUST run, still using that scenario's other configuration.

#### Scenario: Named exec target wins over an exported default function
- **GIVEN** a script that exports a default function
- **AND** declares one scenario whose exec target is a different exported function
- **WHEN** the user runs it with `--once`
- **THEN** the scenario's exec target runs exactly once
- **AND** the default function does not run even though the script exports it

#### Scenario: Scenario without an exec target falls back to the default function
- **GIVEN** a script with one scenario that names no exec target, or names the default function, and sets environment variables, tags, and browser options
- **WHEN** the user runs it with `--once`
- **THEN** the default function runs exactly once
- **AND** it runs with that scenario's environment variables, tags, and browser options

### Requirement: Multiple scenarios are rejected

When a script declares two or more scenarios, single-run mode MUST fail, because it cannot choose which scenario to run or whose configuration to use without ambiguity. The system MUST NOT pick one of them, and MUST NOT run several of them together. The error MUST name the available scenarios. The rejection MUST happen before the script runs.

#### Scenario: Multi-scenario script errors before anything runs
- **GIVEN** a script declaring three scenarios
- **WHEN** the user runs it with `--once` under `k6 run`
- **AND** separately under `k6 cloud run`
- **THEN** each invocation fails, naming the three scenario names
- **AND** the process exits with a non-zero code

### Requirement: Scripts with no runnable entry point fail

When a script declares neither scenarios nor a default function, single-run mode MUST fail with the same missing-default-function error k6 already reports for such a script today.

#### Scenario: Nothing to run
- **GIVEN** a script with no scenarios and no exported default function
- **WHEN** the user runs it with `--once`
- **THEN** k6 reports the same missing-default-function error it reports for that script without the flag
- **AND** the process exits with a non-zero code

### Requirement: Load-shaping command-line flags cannot be combined with single-run mode

The virtual-users flag, the iterations flag, the duration flag, and the stages flag MUST each be rejected when combined with `--once` on the same command line. The error MUST name the conflicting flag. The rejection MUST happen before the script runs, even when the script would otherwise be accepted by single-run mode. The system MUST NOT resolve the ambiguity by preferring one of the two flags.

#### Scenario: Combination is refused even for an otherwise valid script
- **GIVEN** a script with exactly one scenario, which single-run mode would accept on its own
- **WHEN** the user runs it with `--once` together with the iterations flag
- **AND** separately with `--once` together with the virtual-users flag
- **AND** separately with `--once` together with the duration flag
- **AND** separately with `--once` together with the stages flag
- **AND** separately with each of those combinations under `k6 cloud run`
- **THEN** each invocation reports that single-run mode cannot be combined with that flag
- **AND** the script does not run
- **AND** the process exits with a non-zero code

### Requirement: Browser tests run correctly under single-run mode

Because scenario-level browser options survive single-run mode, a browser script MUST run its browser steps successfully without the user editing the script or passing load-shaping flags. A browser MUST be launched only when the scenario that actually runs declares browser options.

#### Scenario: Browser scenario runs once with its browser
- **GIVEN** a script with one scenario that declares browser options and drives a page
- **WHEN** the user runs it with `--once`
- **THEN** the browser is launched and the scenario runs exactly one iteration on one virtual user
- **AND** the run does not fail for a missing browser

#### Scenario: No browser launches for a non-browser scenario
- **GIVEN** a script with one scenario that declares no browser options
- **WHEN** the user runs it with `--once`
- **THEN** no browser is launched

### Requirement: Single-run mode is a command-line flag only

Single-run mode MUST turn on only when `--once` appears on that invocation's command line. An environment variable named after the flag MUST NOT turn single-run mode on, and a configuration-file field named after the flag MUST NOT turn it on. In both cases the script MUST run with its declared load. This keeps a stray setting left in a shell profile or a shared configuration file from turning a real load test into a single run.

#### Scenario: Environment variable and configuration file cannot turn on single-run mode
- **GIVEN** a script whose scenario declares 100 virtual users and 100 iterations
- **AND** an environment variable named after the `--once` flag is set
- **AND** the configuration file contains a field named after the `--once` flag
- **WHEN** the user runs the script without `--once` on the command line
- **THEN** the script runs with its declared 100 virtual users and 100 iterations

### Requirement: Setup and teardown are unaffected

Single-run mode MUST change only the scenario configuration. The script's setup and teardown functions MUST behave exactly as they do without the flag, each running once around the single iteration.

#### Scenario: Setup and teardown still run around the single iteration
- **GIVEN** a script that exports setup and teardown functions
- **WHEN** the user runs it with `--once`
- **THEN** setup runs exactly once before the iteration
- **AND** teardown runs exactly once after the iteration

### Requirement: Archives carry single-run mode forward

An archive generated for a cloud run under single-run mode MUST contain the resulting 1 virtual user / 1 iteration scenario, so that re-running that archive later, including from the cloud interface, also runs exactly once.

#### Scenario: Re-running a generated archive still runs once
- **GIVEN** a script whose scenario declares 100 virtual users and 100 iterations
- **WHEN** the user starts a cloud run with `--once`
- **AND** the archive produced for that run is run again later without the flag
- **THEN** the later run also runs exactly one iteration on one virtual user
