## ADDED Requirements

### Requirement: Optional scenario name on the single-run flag

The `--once` flag MUST accept an optional scenario name supplied on the flag itself, and that name MUST select which of the script's scenarios runs, under every command and execution mode the flag is accepted on. The system MUST NOT run more than one scenario in a single invocation under any circumstance, whether a name is given or not, and scenarios the user did not name MUST NOT run. Supplying the flag with an empty name MUST behave exactly as the bare flag. When a script declares two or more scenarios and no name is given, single-run mode MUST still refuse to run the script, and its error MUST tell the user that a scenario name is required with the flag and MUST list the available scenario names, so that the error itself teaches the user the name argument exists. This wording replaces the multi-scenario error wording the single-run capability requires: once scenario naming ships, that error MUST ask for a scenario name rather than only reporting that single-run mode needs a single scenario.

#### Scenario: Naming a scenario runs that scenario alone
- **GIVEN** a script declaring three scenarios named `api`, `ui`, and `ts`, each with its own load
- **WHEN** the user runs it with `--once=api`
- **THEN** the `api` scenario runs for exactly one iteration on exactly one virtual user
- **AND** the `ui` and `ts` scenarios do not run

#### Scenario: Bare flag on a multi-scenario script asks for a name
- **GIVEN** a script declaring three scenarios named `api`, `ui`, and `ts`
- **WHEN** the user runs it with bare `--once`, giving no scenario name
- **AND** separately with `--once=`, giving an empty name
- **THEN** each invocation reports that a scenario name is required with the flag when the script declares several scenarios
- **AND** the error lists `api`, `ui`, and `ts` as the available scenario names
- **AND** no scenario runs
- **AND** the process exits with a non-zero code

### Requirement: The named scenario alone supplies the run's configuration

The run MUST use the named scenario's exec target, browser options, environment variables, and tags, and MUST ignore the configuration of every other scenario in the script, including its browser options. A browser MUST be launched only when the named scenario itself declares browser options. When the named scenario declares an exec target, that target MUST run and the default function MUST NOT run, even when the script exports one.

#### Scenario: Browser options and exec target come from the named scenario only
- **GIVEN** a script that exports a default function
- **AND** declares a scenario named `api` with its own exec target, environment variables, tags, and no browser options
- **AND** declares a scenario named `ui` with its own exec target and browser options, driving a page
- **WHEN** the user runs it with `--once=ui`
- **AND** separately with `--once=api`
- **THEN** the `--once=ui` run launches a browser, runs the `ui` scenario's exec target for exactly one iteration on one virtual user, and does not fail for a missing browser
- **AND** the `--once=api` run uses the `api` scenario's exec target, environment variables, and tags, and launches no browser, even though the `ui` scenario declares browser options
- **AND** neither run runs the default function, because each named scenario declares its own exec target

### Requirement: A named scenario without an exec target runs the default function

When the named scenario declares no exec target, or names the default function as its target, the default function MUST run, using that scenario's other configuration.

#### Scenario: Named scenario falls back to the default function
- **GIVEN** a script that exports a default function
- **AND** declares a scenario named `ts` that names no exec target and sets environment variables, tags, and browser options
- **WHEN** the user runs it with `--once=ts`
- **THEN** the default function runs exactly once on one virtual user
- **AND** it runs with the `ts` scenario's environment variables, tags, and browser options

### Requirement: A name that selects nothing is rejected before the script runs

When the given name matches no scenario the script declares, k6 MUST fail with an error that names the scenario that was not found and lists the scenario names the script does declare, whatever the number of scenarios the script actually declares -- none, exactly one, or several. When the script declares no scenarios at all, naming one MUST also fail, with an error saying the script declares no scenarios, and MUST NOT fall back to running the default function. Both failures MUST happen before the script runs, and the process MUST exit with a non-zero code. When more than one reason to reject the invocation applies at once, such as an unknown name given together with a load-shaping flag, reporting any one of those reasons is sufficient, and the script MUST NOT run.

#### Scenario: Unknown name fails early and says what exists
- **GIVEN** a script declaring scenarios named `api`, `ui`, and `ts`
- **WHEN** the user runs it with `--once=foo`
- **AND** separately, given a script declaring exactly one scenario named `api`, when the user runs it with `--once=foo`
- **THEN** each invocation reports that the scenario `foo` was not found and lists the scenario names the script actually declares
- **AND** nothing runs
- **AND** the process exits with a non-zero code

#### Scenario: Naming a scenario in a script that has none
- **GIVEN** a script that declares no scenarios and exports a default function
- **WHEN** the user runs it with `--once=api`
- **THEN** k6 reports that the script declares no scenarios
- **AND** the default function does not run
- **AND** the process exits with a non-zero code

### Requirement: Naming a scenario changes only which scenario is selected

Supplying a name MUST change nothing about single-run mode except which scenario it selects: every requirement of the single-run capability MUST still apply, unchanged, to the named form. Only the command line MUST be able to turn single-run mode on, so an environment variable or a configuration-file field named after the flag MUST NOT turn it on, whether or not its value carries a scenario name.

#### Scenario: A scenario name outside the command line does not turn on single-run mode
- **GIVEN** a script declaring scenarios named `api` and `ui`, where `api` declares 100 virtual users and 100 iterations
- **AND** an environment variable named after the `--once` flag is set to `api`
- **AND** the configuration file contains a field named after the flag set to `api`
- **WHEN** the user runs the script without `--once` on the command line
- **THEN** single-run mode does not turn on
- **AND** the script runs with its declared load, including 100 virtual users and 100 iterations for `api`
