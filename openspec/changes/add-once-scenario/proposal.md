## Why

Single-run mode refuses to run any script that declares more than one scenario, so users with real multi-scenario load-testing scripts cannot smoke-test one of them without editing the script. Letting the user name the scenario on the same flag removes that last edit, and since `k6 inspect` already lists a script's scenario names, users can find out what to pass without new tooling.

## What Changes

- `--once` accepts an optional scenario name on the same flag, so `--once=api` runs the scenario named `api`.
- Naming a scenario works the same way under `k6 run`, under `k6 cloud run`, under `k6 cloud run --local-execution`, and when running an already-built archive.
- Exactly one scenario runs per invocation; the flag never runs several scenarios together.
- The named scenario supplies the configuration for the run: its exec target, its browser options, its environment variables, and its tags. Other scenarios contribute nothing, including their browser options.
- When the named scenario declares an exec target, that target runs and the default function does not, even for a script that exports one.
- A named scenario that declares no exec target, or names the default function, runs the default function with that scenario's other configuration.
- A name that matches no scenario fails before anything runs, with an error that names the missing scenario and lists the scenario names the script does declare, whether the script declares one scenario, several, or none at all.
- Bare `--once` on a script with several scenarios still refuses to run, and its error now asks for a scenario name alongside the list of available names. This replaces the wording the single-run capability requires today for that same case, which only reported that single-run mode needs a single scenario.
- `--once=` with an empty name behaves exactly as the bare flag.

## Capabilities

### New Capabilities

- once-scenario: Lets the user name which scenario single-run mode runs, by passing the scenario name on the `--once` flag itself.

### Modified Capabilities

- once: Its multi-scenario rejection now asks the user for a scenario name, replacing the error wording that only reported that single-run mode needs a single scenario.

## Impact

- The `run` and `cloud run` command-line surfaces, including local execution and running an already-built archive.
- The single-run scenario configuration layer, including the error it reports for a script with several scenarios, whose wording this change replaces.
