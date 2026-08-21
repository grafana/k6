## ADDED Requirements

### Requirement: The `--once` flag and what it produces

The system MUST provide a `--once` command-line flag that runs a test exactly one time with 1 virtual user and 1 iteration, with no edit to the script.

Under single-run mode the scenario that runs MUST end up as a `shared-iterations` scenario with `vus: 1` and `iterations: 1`, whatever executor the test declared, and with no exception for any configuration source, flag, or option. The system MUST NOT run more than one scenario under single-run mode.

`--once` MUST be accepted by `k6 run`, `k6 cloud run`, `k6 cloud run --local-execution`, and by `k6 run` on an already-built archive. It MUST NOT be accepted by `k6 archive` or `k6 cloud upload`, the two commands that build an archive without running it.

`--once` MUST take no value. As a boolean flag it also accepts `--once=true` and `--once=false`, and `--once=false` MUST leave single-run mode off, so the test runs its declared load. Any other value MUST be rejected as an invalid flag value and MUST NOT be read as a scenario name: naming a scenario to run is a separate, later change, and silently treating `--once=api` as a bare `--once` would run the wrong scenario.

Note for whoever writes the tests: k6 prints the scenario description once, in the block before the run starts, and `--quiet` suppresses that block. The executed iteration count is a separate line, `N complete and 0 interrupted iterations`. Do not match the padded `iterations...........:` summary line — the dot run grows with the longest metric name in the test.

#### Scenario: A 100 VU / 100 iteration scenario runs once
- **GIVEN** `api.js`:
  ```js
  export const options = { scenarios: { api: {
    executor: 'shared-iterations', exec: 'api', vus: 100, iterations: 100,
  }}};
  export function api() { console.log('api ran'); }
  ```
- **WHEN** the user runs `k6 run --once api.js`
- **THEN** the pre-run scenario line reads `* api: 1 iterations shared among 1 VUs (maxDuration: 10m0s, exec: api, gracefulStop: 30s)`
- **AND** stdout contains `1 complete and 0 interrupted iterations`
- **AND** `api ran` is printed exactly once
- **AND** the process exits with code 0

#### Scenario: A ramping scenario becomes shared-iterations
- **GIVEN** `ramp.js`:
  ```js
  export const options = { scenarios: { api: {
    executor: 'ramping-vus', exec: 'api', startVUs: 10,
    stages: [{ duration: '30s', target: 100 }], gracefulRampDown: '10s',
  }}};
  export function api() { console.log('api ran'); }
  ```
- **WHEN** the user runs `k6 run --once ramp.js`
- **THEN** the pre-run scenario line reads `* api: 1 iterations shared among 1 VUs (maxDuration: 10m0s, exec: api, gracefulStop: 30s)`
- **AND** it is no longer the ramping description `* api: Up to 100 looping VUs for 30s over 1 stages (gracefulRampDown: 10s, exec: api, gracefulStop: 30s)` that k6 prints without the flag
- **AND** stdout contains `1 complete and 0 interrupted iterations`

#### Scenario: Running an already-built archive
- **GIVEN** an archive built with `k6 archive api.js -O api.tar`, whose stored options declare the 100 VU / 100 iteration `api` scenario
- **WHEN** the user runs `k6 run --once api.tar`
- **THEN** stdout contains `1 complete and 0 interrupted iterations`
- **AND** `api ran` is printed exactly once

#### Scenario: Local execution against the cloud
- **GIVEN** `api.js`
- **WHEN** the user runs `k6 cloud run --local-execution --once api.js`
- **THEN** stdout contains `1 complete and 0 interrupted iterations`

#### Scenario: Cloud execution
- **GIVEN** `api.js`
- **WHEN** the user runs `k6 cloud run --once api.js`
- **THEN** the options k6 sends when it creates the cloud test run declare one scenario, named `api`, with `vus: 1` and `iterations: 1`

#### Scenario: The archive-building commands reject the flag
- **WHEN** the user runs `k6 archive --once api.js`
- **AND** separately `k6 cloud upload --once api.js`
- **THEN** each reports `unknown flag: --once`
- **AND** each exits with a non-zero code

#### Scenario: `--once=api` is not a scenario selector
- **GIVEN** `multi.js`
- **WHEN** the user runs `k6 run --once=api multi.js`
- **THEN** k6 rejects the value as an invalid boolean, naming the `once` flag
- **AND** the process exits with a non-zero code
- **AND** it does not fall back to single-run mode and does not run any scenario

#### Scenario: `--once=false` leaves the declared load alone
- **GIVEN** `api.js`
- **WHEN** the user runs `k6 run --once=false api.js`
- **THEN** the pre-run scenario line reads `* api: 100 iterations shared among 100 VUs (maxDuration: 10m0s, exec: api, gracefulStop: 30s)`
- **AND** stdout contains `100 complete and 0 interrupted iterations`

### Requirement: The running scenario keeps its identity and its non-load configuration

Single-run mode MUST preserve the running scenario's name, its `exec` target, its `options` block including `options.browser`, its `env`, and its `tags`.

The name matters beyond display: it is a system tag on every metric the run emits, it is what `k6/execution` reports as `scenario.name`, and it is the key the browser module uses to find `options.browser`. A single-run scenario MUST NOT be renamed.

#### Scenario: Name, exec target, env, and tags all survive
- **GIVEN** `keep.js`:
  ```js
  import exec from 'k6/execution';
  export const options = { scenarios: { api: {
    executor: 'shared-iterations', exec: 'api', vus: 50, iterations: 50,
    env: { URL: 'https://example.com' }, tags: { team: 'core' },
  }}};
  export function api() { console.log(`name=${exec.scenario.name} url=${__ENV.URL}`); }
  export default function () { console.log('default ran'); }
  ```
- **WHEN** the user runs `k6 run --once --out json=out.json keep.js`
- **THEN** `name=api url=https://example.com` is printed exactly once
- **AND** `default ran` is not printed
- **AND** every metric point in `out.json` carries the tags `scenario: api` and `team: core`

### Requirement: Load-shaping configuration is discarded

Single-run mode MUST discard the running scenario's load-shaping configuration: `executor`, `vus`, `iterations`, `duration`, `startTime`, `maxDuration`, `timeUnit`, `stages`, `rate`, `gracefulStop`, `gracefulRampDown`, `preAllocatedVUs`, and every other field belonging to the declared executor, such as `startVUs` and `maxVUs`.

The list is not the mechanism. The resulting scenario MUST be a fresh `shared-iterations` configuration, so no field of the declared executor can survive by omission from the list. k6 rejects a scenario carrying a field its executor does not own, so an implementation that deletes only the listed fields produces a scenario k6 cannot load.

Discarded means replaced, not merged: the resulting scenario carries the `shared-iterations` defaults for anything it does not preserve. A declared `maxDuration` or `gracefulStop` therefore does not carry over, and the single iteration runs under the default budget of 10 minutes with a 30 second graceful stop.

#### Scenario: Arrival-rate shaping has no effect
- **GIVEN** `rate.js`:
  ```js
  export const options = { scenarios: { api: {
    executor: 'constant-arrival-rate', exec: 'api',
    rate: 100, timeUnit: '1s', duration: '10m',
    preAllocatedVUs: 50, maxVUs: 200, startTime: '30s',
  }}};
  export function api() { console.log('api ran'); }
  ```
- **WHEN** the user runs `k6 run --once rate.js`
- **THEN** the pre-run scenario line reads `* api: 1 iterations shared among 1 VUs (maxDuration: 10m0s, exec: api, gracefulStop: 30s)`
- **AND** `api ran` is printed within a second of start, not after the declared 30 second `startTime`
- **AND** stdout contains `1 complete and 0 interrupted iterations`

#### Scenario: A declared maximum duration and graceful stop do not carry over
- **GIVEN** `budget.js`:
  ```js
  export const options = { scenarios: { api: {
    executor: 'shared-iterations', exec: 'api', vus: 5, iterations: 5,
    maxDuration: '1h', gracefulStop: '5m',
  }}};
  export function api() { console.log('api ran'); }
  ```
- **WHEN** the user runs `k6 run --once budget.js`
- **THEN** the pre-run scenario line reads `* api: 1 iterations shared among 1 VUs (maxDuration: 10m0s, exec: api, gracefulStop: 30s)`
- **AND** the pre-run header reads `1 scenario, 1 max VUs, 10m30s max duration (incl. graceful stop):`, not the `1h5m30s` the declared values would give
- **AND** stdout contains `1 complete and 0 interrupted iterations`

### Requirement: A test with no scenarios runs its default function

When a test declares no scenarios and exports a `default` function, single-run mode MUST run that function once, as a `shared-iterations` scenario named `default` with `vus: 1` and `iterations: 1`. Load the test declares outside a scenario MUST NOT take effect.

#### Scenario: A default-only script runs once
- **GIVEN** `plain.js`:
  ```js
  export const options = { vus: 50, duration: '30s' };
  export default function () { console.log('default ran'); }
  ```
- **WHEN** the user runs `k6 run --once plain.js`
- **THEN** the pre-run scenario line reads `* default: 1 iterations shared among 1 VUs (maxDuration: 10m0s, gracefulStop: 30s)`
- **AND** it is no longer the `* default: 50 looping VUs for 30s (gracefulStop: 30s)` that k6 prints without the flag
- **AND** `default ran` is printed exactly once
- **AND** stdout contains `1 complete and 0 interrupted iterations`

### Requirement: One declared scenario is the one that runs

When a test declares exactly one scenario, single-run mode MUST run that scenario, whatever it is named and wherever it was declared.

If the scenario names an `exec` target, that target MUST run and the exported `default` function MUST NOT run, even when the script exports one. If the scenario names no `exec` target, or names `default`, then the `default` function MUST run, still under that scenario's preserved configuration. Such a scenario needs a `default` export: without one the run MUST fail.

#### Scenario: A scenario with no exec target runs the default function
- **GIVEN** `ui.js`:
  ```js
  export const options = { scenarios: { ui: {
    executor: 'constant-vus', vus: 10, duration: '30s',
    env: { BASE_URL: 'https://example.com' },
  }}};
  export default function () { console.log(`default ran url=${__ENV.BASE_URL}`); }
  ```
- **WHEN** the user runs `k6 run --once ui.js`
- **THEN** `default ran url=https://example.com` is printed exactly once
- **AND** the pre-run scenario line names the scenario `ui`, not `default`
- **AND** stdout contains `1 complete and 0 interrupted iterations`

#### Scenario: `exec: 'default'` behaves the same as no exec target
- **GIVEN** `uiexec.js`, which is `ui.js` with `exec: 'default'` added to the `ui` scenario
- **WHEN** the user runs `k6 run --once uiexec.js`
- **THEN** `default ran url=https://example.com` is printed exactly once
- **AND** the pre-run scenario line names the scenario `ui`
- **AND** stdout contains `1 complete and 0 interrupted iterations`

#### Scenario: A scenario needing the default function fails without one
- **GIVEN** `nodefault.js`, which is `ui.js` with its `export default` replaced by `export function api() {}`, so the `ui` scenario still targets `default` but nothing exports it
- **WHEN** the user runs `k6 run --once nodefault.js`
- **THEN** standard error contains `executor ui: function 'default' not found in exports`
- **AND** the process exits with a non-zero code
- **AND** it is the same message and code that `k6 run nodefault.js` gives without the flag

#### Scenario: The single scenario may come from a configuration file
- **GIVEN** `plaindef.js`, which declares no scenarios and exports `default`
- **AND** `cfg.json` containing exactly:
  ```json
  {"scenarios": {"api": {"executor": "shared-iterations", "vus": 7, "iterations": 9, "tags": {"team": "core"}}}}
  ```
- **WHEN** the user runs `k6 run --once -c cfg.json --out json=out.json plaindef.js`
- **THEN** the pre-run scenario line names the scenario `api` and reads `1 iterations shared among 1 VUs`
- **AND** stdout contains `1 complete and 0 interrupted iterations`
- **AND** every metric point in `out.json` carries the tags `scenario: api` and `team: core`

### Requirement: Declared load never takes effect, whatever declared it

Single-run mode MUST reach 1 virtual user and 1 iteration whatever source declared the load, and the running scenario's non-load configuration MUST survive that. There is no exception. The sources are:

- the script's `options`, both inside a scenario and at the top level
- the environment variables `K6_VUS`, `K6_ITERATIONS`, `K6_DURATION`, and `K6_STAGES`
- the JSON configuration file, whether named with `-c` or picked up from its default location
- an archive's stored options, when `k6 run` is given an archive

The command line is the one source that is refused outright rather than neutralized, because a load flag on the same command line as `--once` is a contradiction the user can see and fix. Every other source is silently overridden, because the user may not control it.

This is the requirement with real weight behind it. k6 consolidates its sources before single-run mode acts, and any source that sets `vus`, `iterations`, `duration`, or `stages` throws the whole `scenarios` map away while consolidating, taking `exec`, `env`, `tags`, and `options.browser` with it. Today all four environment variables above turn the browser script below into `executor default: function 'default' not found in exports` with exit 104. Under `--once` that script MUST run once, with its scenario intact. Satisfying this reaches into how k6 merges configuration, and that is accepted: without it, a stray variable in a shell profile or a probe's environment silently defeats the flag, which is the failure this whole change exists to remove.

#### Scenario: An environment variable cannot defeat single-run mode
- **GIVEN** `envkeep.js`:
  ```js
  import exec from 'k6/execution';
  export const options = { scenarios: { api: {
    executor: 'shared-iterations', exec: 'api', vus: 50, iterations: 50,
    env: { URL: 'https://example.com' }, tags: { team: 'core' },
    options: { browser: { type: 'chromium' } },
  }}};
  export function api() { console.log(`name=${exec.scenario.name} url=${__ENV.URL}`); }
  ```
- **WHEN** the user runs `k6 run --once envkeep.js` with `K6_VUS=100` set
- **AND** separately with `K6_ITERATIONS=100`
- **AND** separately with `K6_DURATION=30s`
- **AND** separately with `K6_STAGES=10s:5`
- **THEN** each run prints `name=api url=https://example.com` exactly once
- **AND** each pre-run scenario line names the scenario `api`, not `default`
- **AND** stdout contains `1 complete and 0 interrupted iterations`
- **AND** no warning contains `overrides scenarios configuration` or `overrode scenarios configuration entirely`
- **AND** each run exits with code 0, where without `--once` all four exit 104 with `executor default: function 'default' not found in exports`

#### Scenario: A configuration file cannot defeat single-run mode
- **GIVEN** `envkeep.js` and `load.json` containing exactly `{"vus": 100, "iterations": 100}`
- **WHEN** the user runs `k6 run --once -c load.json envkeep.js`
- **THEN** `name=api url=https://example.com` is printed exactly once
- **AND** the pre-run scenario line names the scenario `api`
- **AND** stdout contains `1 complete and 0 interrupted iterations`

#### Scenario: An archive's stored load cannot defeat single-run mode
- **GIVEN** `load.tar`, an archive whose stored options carry both the `api` scenario and a top-level `iterations` value
- **WHEN** the user runs `k6 run --once load.tar`
- **THEN** stdout contains `1 complete and 0 interrupted iterations`
- **AND** the run does not fail with a conflict between top-level options and `scenarios`

### Requirement: Two or more scenarios are rejected

When a test declares two or more scenarios, single-run mode MUST fail, because it cannot pick one of them or combine their configurations without ambiguity. It MUST NOT pick one, and MUST NOT run several.

The error MUST be reported at error level and MUST contain `the --once flag can run only a single scenario`. It MUST be a single line, so one substring match works whatever the log format. It MUST NOT list the declared scenario names: Proposal 1's error carries no names, listing them is Proposal 2's affordance (`available scenarios: api, ui, ts`), and offering the list here implies a choice the flag does not accept.

The rejection MUST happen before any iteration starts and before `setup()` runs. The script's init context still runs, because that is where k6 reads the scenario list from.

#### Scenario: A three-scenario script is rejected before anything runs
- **GIVEN** `multi.js`:
  ```js
  console.log('init ran');
  export const options = { scenarios: {
    zulu: { executor: 'shared-iterations', exec: 'zulu' },
    xray: { executor: 'shared-iterations', exec: 'xray' },
    kilo: { executor: 'shared-iterations', exec: 'kilo', options: { browser: { type: 'chromium' } } },
  }};
  export function setup() { console.log('setup ran'); }
  export function zulu() { console.log('zulu ran'); }
  export function xray() { console.log('xray ran'); }
  export async function kilo() { const p = await browser.newPage(); await p.close(); }
  ```
  with `import { browser } from 'k6/browser';` at the top, so the browser scenario really would launch Chromium if it ran
- **WHEN** the user runs `k6 run --once multi.js`
- **THEN** standard error contains `the --once flag can run only a single scenario`
- **AND** the message contains none of `zulu`, `xray`, or `kilo`
- **AND** `setup ran`, `zulu ran`, and `xray ran` are all absent from the output
- **AND** no Chromium process is started
- **AND** `init ran` is present, because k6 reads the scenario list from the init context
- **AND** the process exits with a non-zero code

#### Scenario: The same rejection under cloud run
- **GIVEN** `multi.js`
- **WHEN** the user runs `k6 cloud run --once multi.js`
- **THEN** standard error contains `the --once flag can run only a single scenario`
- **AND** k6 sends no request to create a cloud test run
- **AND** the process exits with a non-zero code

### Requirement: A test with nothing to run fails as it already does

When a test declares neither scenarios nor a `default` function, single-run mode MUST fail exactly as k6 fails for that same test without the flag: the same message and the same exit code.

#### Scenario: Nothing to run
- **GIVEN** `noentry.js` containing only `export function api() {}`
- **WHEN** the user runs `k6 run --once noentry.js`
- **THEN** standard error contains `executor default: function 'default' not found in exports`
- **AND** the message and the exit code are the same ones `k6 run noentry.js` gives without the flag

### Requirement: Load-shaping command-line flags cannot be combined with `--once`

`--vus`/`-u`, `--iterations`/`-i`, `--duration`/`-d`, and `--stage`/`-s` MUST each be rejected when passed together with `--once`, in their long and their short form alike. The rejection MUST happen before any iteration starts, even for a test single-run mode would otherwise accept, and the system MUST NOT resolve the ambiguity by preferring either flag.

The error MUST name both flags. k6 has no existing "cannot be combined" wording, so this follows the flag-conflict phrasing k6 already uses for `--local-execution`: `the --once flag is not compatible with the --iterations flag`, written to standard error, with the same shape for `--vus`, `--duration`, and `--stage`. A test may match on the two flag names appearing in one error line rather than on the whole sentence.

#### Scenario: `--iterations` is refused
- **GIVEN** `api.js`, which single-run mode accepts on its own
- **WHEN** the user runs `k6 run --once --iterations 10 api.js`
- **THEN** standard error contains `the --once flag is not compatible with the --iterations flag`
- **AND** `api ran` is not printed
- **AND** the process exits with a non-zero code

#### Scenario: `--vus` is refused
- **WHEN** the user runs `k6 run --once --vus 10 api.js`
- **THEN** standard error contains `the --once flag is not compatible with the --vus flag`, `api ran` is not printed, and the process exits with a non-zero code

#### Scenario: `--duration` is refused
- **WHEN** the user runs `k6 run --once --duration 30s api.js`
- **THEN** standard error contains `the --once flag is not compatible with the --duration flag`, `api ran` is not printed, and the process exits with a non-zero code

#### Scenario: `--stage` is refused
- **WHEN** the user runs `k6 run --once --stage 30s:10 api.js`
- **THEN** standard error contains `the --once flag is not compatible with the --stage flag`, `api ran` is not printed, and the process exits with a non-zero code

#### Scenario: The short forms are refused too
- **WHEN** the user runs `k6 run --once -i 10 api.js`
- **AND** separately `k6 run --once -u 10 api.js`
- **AND** separately `k6 run --once -d 30s api.js`
- **AND** separately `k6 run --once -s 30s:10 api.js`
- **THEN** each names the conflicting flag on standard error and exits with a non-zero code

#### Scenario: The rejection also applies to cloud run
- **WHEN** the user runs `k6 cloud run --once -i 10 api.js`
- **THEN** standard error names the conflict, k6 sends no request to create a cloud test run, and the process exits with a non-zero code

### Requirement: Browser tests run under single-run mode

Because the running scenario keeps its name and its `options.browser` block, a browser test MUST run its browser steps under `--once` with no script edit and no load-shaping flag. This is the case that shortcut flags break today.

A browser MUST be launched only when the running scenario declares `options.browser`.

#### Scenario: A browser scenario runs once, with its browser
- **GIVEN** `browser.js`:
  ```js
  import { browser } from 'k6/browser';
  export const options = { scenarios: { ui: {
    executor: 'shared-iterations', vus: 10, iterations: 10,
    options: { browser: { type: 'chromium' } },
  }}};
  export default async function () {
    const page = await browser.newPage();
    try {
      await page.goto('https://test.k6.io/', { waitUntil: 'networkidle' });
      console.log(`title=${await page.title()}`);
    } finally { await page.close(); }
  }
  ```
- **WHEN** the user runs `k6 run --once browser.js`
- **THEN** the summary contains a `browser_web_vital_lcp` line, so a real browser drove a real page
- **AND** `browser not found in registry` is absent from the output
- **AND** stdout contains `1 complete and 0 interrupted iterations`

#### Scenario: No browser launches when the scenario declares none
- **GIVEN** `nobrowser.js`, which is `browser.js` with the `options.browser` block deleted
- **WHEN** the user runs `k6 run --once nobrowser.js`
- **THEN** no `browser_` metric appears in the summary
- **AND** standard error contains `browser not found in registry`, exactly as it does for that script without the flag

### Requirement: Single-run mode is a command-line flag only

Single-run mode MUST turn on only from `--once` on the invocation's own command line. No environment variable and no configuration-file field may turn it on. Concretely, `K6_ONCE`, a bare `ONCE`, and a `once` field in the JSON configuration file MUST all leave single-run mode off.

k6 decodes environment configuration with an empty prefix, so an unprefixed variable named after a configuration field reaches the decoder. Both spellings therefore have to be inert, not just the prefixed one. The point of the rule is that a setting left in a shell profile or a shared configuration file can never turn a real load test into a single run, where passing looks like success.

#### Scenario: Neither environment variable turns it on
- **GIVEN** `api.js`
- **AND** `K6_ONCE=true` and `ONCE=true` are both set
- **WHEN** the user runs `k6 run api.js`, with no `--once`
- **THEN** the pre-run scenario line reads `* api: 100 iterations shared among 100 VUs (maxDuration: 10m0s, exec: api, gracefulStop: 30s)`
- **AND** stdout contains `100 complete and 0 interrupted iterations`, so the declared load really ran
- **AND** the process exits with code 0

#### Scenario: The configuration file cannot turn it on
- **GIVEN** `api.js` and `once.json` containing exactly `{"once": true}`
- **WHEN** the user runs `k6 run -c once.json api.js`
- **THEN** stdout contains `100 complete and 0 interrupted iterations`
- **AND** the process exits with code 0

### Requirement: Setup and teardown are unaffected

Single-run mode changes scenario configuration only. `setup()` and `teardown()` MUST behave exactly as they do without the flag on any invocation single-run mode accepts, including the flags that skip them.

#### Scenario: Setup and teardown run once around the single iteration
- **GIVEN** `lifecycle.js` exporting `setup()`, `teardown()`, and a `default` function, each printing a distinct line
- **WHEN** the user runs `k6 run --once lifecycle.js`
- **THEN** the output shows the setup line, then the default line, then the teardown line, in that order
- **AND** each appears exactly once

#### Scenario: The skip flags still skip
- **GIVEN** `lifecycle.js`
- **WHEN** the user runs `k6 run --once --no-setup --no-teardown lifecycle.js`
- **THEN** neither the setup line nor the teardown line appears
- **AND** the default line appears exactly once

### Requirement: Archives carry single-run mode forward

An archive that k6 builds for a cloud run started with `--once` MUST carry the resulting 1 virtual user / 1 iteration scenario, so a later run of that archive also runs once. This holds for the archive uploaded by `k6 cloud run` and for the archive uploaded by `k6 cloud run --local-execution`.

The archive's stored options MUST hold the single-run scenario and nothing else that shapes load: the top-level `vus`, `iterations`, `duration`, and `stages` values MUST all be absent. This is absolute, not a preference. An archive built under `--once` describes a single run, so re-running it MUST give a single run even without the flag, and any leftover top-level load value breaks that: `iterations`, `duration`, or `stages` makes k6 refuse the archive outright, and `vus` is worse, because it silently discards the single-run scenario and runs that many virtual users while exiting 0.

#### Scenario: Re-running the uploaded archive runs once
- **GIVEN** `api.js`, whose scenario declares 100 virtual users and 100 iterations
- **WHEN** the user starts `k6 cloud run --once api.js`
- **AND** the archive k6 uploaded for that run is later run with `k6 run`, without `--once`
- **THEN** the archive's `metadata.json` options have `vus`, `iterations`, `duration`, and `stages` all null
- **AND** the later run's pre-run scenario line reads `* api: 1 iterations shared among 1 VUs (maxDuration: 10m0s, exec: api, gracefulStop: 30s)`
- **AND** the later run prints no warning containing `overrides scenarios configuration`
- **AND** the later run exits with code 0

#### Scenario: The same holds for local execution
- **GIVEN** `api.js`
- **WHEN** the user starts `k6 cloud run --local-execution --once api.js`
- **AND** the archive k6 uploaded for that run is later run with `k6 run`, without `--once`
- **THEN** the later run's pre-run scenario line names the scenario `api` and reads `1 iterations shared among 1 VUs`
- **AND** the later run exits with code 0

### Requirement: Single-run mode leaves the engine's own options alone

Single-run mode rewrites scenario configuration after k6 has consolidated its configuration sources, and changes nothing else. Options that act on the engine rather than on a scenario MUST keep their normal behavior, and single-run mode MUST NOT special-case them.

So `--paused` still holds the run before its single iteration and releases it on resume. `--execution-segment` and `--execution-segment-sequence` still scale the resulting 1 virtual user / 1 iteration scenario exactly as they scale any such scenario, including down to no work for a segment that owns none. A segment that owns no work reports `0 complete and 0 interrupted iterations`; on current k6 such a run also logs a summary error and still exits 0, which is a pre-existing bug unrelated to this change.

Options that are not scenario configuration MUST survive untouched. `thresholds` in particular MUST still be evaluated over the single iteration and MUST still set the exit code. That matters because a threshold is today the only way to make a single run fail on a failed check, which is what turns `--once` into a functional test. An implementation that replaced the whole options object rather than its `scenarios` field would drop the thresholds, and every single run would exit 0 -- the same silent success that makes broken browser tests look like passes.

#### Scenario: A failing threshold still fails the single run
- **GIVEN** `thresh.js`, which declares `thresholds: { checks: ['rate==1.0'] }`, one scenario with 50 virtual users and 50 iterations, and a body running one check that always fails
- **WHEN** the user runs `k6 run --once thresh.js`
- **THEN** standard error reports that the `checks` threshold was crossed
- **AND** the process exits with code 99, the threshold-failure code
- **AND** stdout contains `1 complete and 0 interrupted iterations`

#### Scenario: A passing threshold leaves the single run green
- **GIVEN** `thresh.js` with its check changed to always pass
- **WHEN** the user runs `k6 run --once thresh.js`
- **THEN** the process exits with code 0

#### Scenario: `--paused` holds the single iteration until resumed
- **GIVEN** `api.js`
- **WHEN** the user runs `k6 run --once --paused --address localhost:6565 api.js`
- **THEN** the progress output reports the run as `paused` and `api ran` is not printed
- **AND** after a `PATCH /v1/status` sets `paused` to false, `api ran` is printed exactly once
- **AND** stdout contains `1 complete and 0 interrupted iterations`
