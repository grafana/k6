## ADDED Requirements

### Requirement: Bare `--once` runs one VU for one iteration

Passing bare `--once` MUST configure exactly one effective `shared-iterations` scenario with `vus: 1` and `iterations: 1`. It MUST work without a script change.

#### Scenario: Primary run commands use the same configuration

- **GIVEN** `api.js` declares one `shared-iterations` scenario named `api`, with `exec: 'api'`, `vus: 4`, and `iterations: 4`, and its `api` function prints `API-RAN`
- **WHEN** the user runs each primary command:

  | Command | Execution |
  | --- | --- |
  | `k6 run --once api.js` | Local |
  | `k6 cloud run --once api.js` | Cloud |
  | `k6 cloud run --local-execution --once api.js` | Local with cloud provisioning |

- **THEN** each local execution lists `api` as `1 iterations shared among 1 VUs`
- **AND** each local execution prints `API-RAN` once and reports `1 complete and 0 interrupted iterations`
- **AND** each cloud upload or provisioning request contains one scenario named `api`, with `executor: shared-iterations`, `vus: 1`, `iterations: 1`, and `exec: api`

### Requirement: Single-run mode creates a fresh `shared-iterations` scenario

`--once` MUST build a new scenario as follows:

| Action | Scenario data |
| --- | --- |
| Preserve | scenario name, `exec`, `env`, `tags`, and the complete `options` block |
| Set | `executor: shared-iterations`, `vus: 1`, and `iterations: 1` |
| Discard | every other field from the original scenario |

Discarded fields include the original `executor`, `vus`, `iterations`, `duration`, `startTime`, `maxDuration`, `timeUnit`, `stages`, `rate`, `startRate`, `gracefulStop`, `gracefulRampDown`, `preAllocatedVUs`, `startVUs`, and `maxVUs`.

`--once` MUST build from the preserved fields instead of copying the original and stripping known load fields. Building from the preserved fields also makes k6 discard fields introduced by future executors.

`--once` MUST leave `maxDuration` and `gracefulStop` unset. k6 MUST serialize them as `null` and apply the normal `shared-iterations` defaults at runtime: 10 minutes and 30 seconds.

#### Scenario: Every built-in executor becomes the same single-run scenario

- **GIVEN** six scripts that each declare one scenario named `api`
- **AND** each `api` function prints its effective scenario from `k6/execution`
- **AND** the declared scenarios use these valid configurations:

  | Executor | Declared load |
  | --- | --- |
  | `shared-iterations` | `vus: 8`, `iterations: 20`, `maxDuration: '1h'`, `gracefulStop: '5m'` |
  | `per-vu-iterations` | `vus: 8`, `iterations: 20`, `maxDuration: '1h'` |
  | `constant-vus` | `vus: 8`, `duration: '30s'`, `startTime: '30s'` |
  | `ramping-vus` | `startVUs: 8`, one stage, `gracefulRampDown: '10s'` |
  | `constant-arrival-rate` | `rate: 100`, `timeUnit: '1s'`, `duration: '30s'`, `preAllocatedVUs: 10`, `maxVUs: 20` |
  | `ramping-arrival-rate` | `startRate: 10`, `timeUnit: '1s'`, one stage, `preAllocatedVUs: 10`, `maxVUs: 20` |

- **WHEN** the user runs each script with `k6 run --once`
- **THEN** each `api` function prints a scenario named `api` with `executor: shared-iterations`, `vus: 1`, and `iterations: 1`
- **AND** `startTime`, `maxDuration`, and `gracefulStop` are `null`
- **AND** each startup summary shows `maxDuration: 10m0s` and `gracefulStop: 30s`
- **AND** each run starts without the declared delay and completes one iteration
- **AND** no effective scenario contains an old load value or a field that `shared-iterations` does not own

### Requirement: The scenario keeps its identity and options

`--once` MUST preserve the scenario name, `exec`, `env`, `tags`, and complete `options` block. Iteration samples MUST use the preserved name and tags. Setup and teardown samples may differ.

#### Scenario: Scenario identity and options survive

- **GIVEN** `keep.js`:

  ```js
  import exec from 'k6/execution';

  export const options = {
    scenarios: {
      api: {
        executor: 'shared-iterations',
        exec: 'api',
        vus: 50,
        iterations: 50,
        env: { URL: 'https://example.com' },
        tags: { team: 'core' },
        options: { browser: { type: 'chromium' } },
      },
    },
  };

  export function api() {
    const scenario = exec.test.options.scenarios.api;
    console.log(`name=${exec.scenario.name} exec=${scenario.exec} executor=${scenario.executor} vus=${scenario.vus} iterations=${scenario.iterations} url=${__ENV.URL} browser=${scenario.options.browser.type}`);
  }

  export default function () { console.log('DEFAULT-RAN'); }
  ```

- **WHEN** the user runs `k6 run --once --out json=out.json keep.js`
- **THEN** the log contains `name=api exec=api executor=shared-iterations vus=1 iterations=1 url=https://example.com browser=chromium` once
- **AND** the log omits `DEFAULT-RAN`
- **AND** every `type: Point` entry emitted by the scenario iteration in `out.json` has `data.tags.scenario: api` and `data.tags.team: core`

### Requirement: One declared scenario chooses the function

With one effective scenario, `--once` MUST run it. A non-empty `exec` MUST select that export. A missing `exec` or `exec: 'default'` MUST select `default`.

#### Scenario: Missing exec and explicit default exec behave the same

- **GIVEN** two versions of a script, each with one scenario named `ui` and `env: { URL: 'https://example.com' }`, declaring either no `exec` or `exec: 'default'`
- **AND** each default function prints `DEFAULT-RAN url=<URL> exec=<effective exec>` from `k6/execution`
- **WHEN** the user runs each script with `k6 run --once`
- **THEN** each run lists a scenario named `ui`, not `default`
- **AND** the missing-exec case prints `DEFAULT-RAN url=https://example.com exec=null` once
- **AND** the explicit case prints `DEFAULT-RAN url=https://example.com exec=default` once

### Requirement: Browser options survive single-run mode

`--once` MUST preserve a declared scenario's name and `options.browser` block.

#### Scenario: A browser scenario keeps its configuration and runs once

- **GIVEN** a controlled local HTTP fixture that responds with HTTP 200 and the title `once fixture`
- **AND** `browser.js` declares one `shared-iterations` scenario named `ui`, with `exec: 'ui'`, 10 VUs, 10 iterations, and `options.browser.type: chromium`
- **AND** `ui` opens the fixture and prints its response status and page title
- **WHEN** the user runs `k6 run --once browser.js`
- **THEN** `status=200 title=once fixture` appears once
- **AND** the summary reports a non-zero `browser_data_received` value
- **AND** k6 does not print `browser not found in registry`
- **AND** the run reports `1 complete and 0 interrupted iterations`

### Requirement: A test without scenarios runs `default`

If the effective configuration has no scenario and the script exports `default`, `--once` MUST create a single-run scenario named `default`. It MUST first clear the top-level `vus`, `iterations`, `duration`, and `stages` fields.

#### Scenario: Every form without scenarios runs `default` once

- **GIVEN** scripts that export a `default` function printing `DEFAULT-RAN` and use one of these option forms:

  | Form | Options |
  | --- | --- |
  | No options | no `options` export |
  | Empty scenarios | `{ scenarios: {} }` |
  | Null scenarios | `{ scenarios: null }` |
  | Virtual-user shortcut | `{ vus: 8 }` |
  | Duration shortcut | `{ vus: 8, duration: '30s' }` |
  | Iteration shortcut | `{ vus: 8, iterations: 20 }` |
  | Stage shortcut | `{ stages: [{ duration: '30s', target: 8 }] }` |

- **WHEN** the user runs each script with `k6 run --once`
- **THEN** k6 lists `default` as `1 iterations shared among 1 VUs`
- **AND** `DEFAULT-RAN` appears once
- **AND** the run reports `1 complete and 0 interrupted iterations`
- **AND** no warning says that a top-level load option overrides `scenarios`

### Requirement: Setup and teardown work normally

`--once` MUST preserve setup and teardown execution, data flow, timeouts, and skip options.

#### Scenario: Setup data reaches the selected function and teardown

- **GIVEN** `lifecycle.js` declares one `shared-iterations` scenario named `api`, with `exec: 'api'`, 50 VUs, and 50 iterations
- **AND** setup prints `SETUP-RAN` and returns `{ token: 'abc' }`
- **AND** `api(data)` prints `API-RAN token=abc`
- **AND** teardown prints `TEARDOWN-RAN token=abc`
- **AND** an exported default function prints `DEFAULT-RAN`
- **WHEN** the user runs `k6 run --once lifecycle.js`
- **THEN** the log shows `SETUP-RAN`, `API-RAN token=abc`, and `TEARDOWN-RAN token=abc` once each and in that order
- **AND** the log omits `DEFAULT-RAN`
- **AND** the run reports one complete iteration

#### Scenario: Existing skip options still skip setup and teardown

- **GIVEN** a lifecycle fixture whose VU function and teardown do not read setup data
- **WHEN** the user runs it with `k6 run --once` under each case:

  | Source | Skip values |
  | --- | --- |
  | Command line | `--no-setup --no-teardown` |
  | JSON config | `noSetup: true, noTeardown: true` |

- **THEN** neither lifecycle marker appears
- **AND** the selected VU function runs once, k6 reports no script exception, and the process exits with code 0

### Requirement: Other options keep normal behavior

`--once` MUST replace only the effective scenario and clear only the top-level `vus`, `iterations`, `duration`, and `stages` fields. Every other consolidated option MUST stay unchanged. Thresholds MUST still determine the exit code. Pause and execution-segment options MUST still apply.

One VU and one iteration describe the configuration, not a promised result. Setup failure, iteration failure, timeout, interruption, pause, and execution segments MUST keep their normal behavior and may prevent completion.

#### Scenario: A failing threshold still fails the single run

- **GIVEN** a test with one 50 VU / 50 iteration scenario, a check that always fails, and `thresholds: { checks: ['rate==1.0'] }`
- **WHEN** the user runs `k6 run --once threshold.js`
- **THEN** the test performs one iteration
- **AND** k6 reports that `checks` crossed its threshold
- **AND** the process exits with the threshold-failure code

#### Scenario: Pause holds the iteration until the user resumes it

- **GIVEN** `api.js` and a free REST API address
- **WHEN** the user runs `k6 run --once --paused --address <address> api.js`
- **THEN** `GET /v1/status` reports `paused: true` and status `4`, while the log omits `API-RAN`
- **AND WHEN** the user sends `PATCH /v1/status` with `{"data":{"type":"status","id":"default","attributes":{"paused":false}}}` and receives HTTP 200
- **THEN** `API-RAN` appears once and the run reports one complete iteration

#### Scenario: Execution segments retain their existing scaling behavior

- **GIVEN** `api.js`
- **WHEN** the user runs `k6 run --once --execution-segment 0:1/2 --execution-segment-sequence 0,1/2,1 api.js`
- **THEN** `API-RAN` appears once and the run reports one complete iteration
- **BUT WHEN** the user runs `k6 run --once --execution-segment 1/2:1 --execution-segment-sequence 0,1/2,1 api.js`
- **THEN** the log omits `API-RAN`, and k6 reports zero VUs and zero complete iterations
- **AND** the process exits with code 0, even if k6 also logs its existing `failed to handle the end-of-test summary` error

### Requirement: All supported run inputs work with `--once`

`k6 run`, `k6 cloud run`, and `k6 cloud run --local-execution` MUST accept `--once` with every script, archive, and standard input source they already support.

#### Scenario: Archives, standard input, and flag placement apply the same configuration

- **GIVEN** `api.js` from the primary run scenario
- **AND** the user builds `api.tar` from `api.js`
- **WHEN** the user uses each supported form:

  | Form | Input |
  | --- | --- |
  | `k6 run --once <input>` | `api.tar`, or `-` with `api.js` or `api.tar` supplied on standard input |
  | `k6 cloud run --once <input>` | `api.tar`, or `-` with `api.js` or `api.tar` supplied on standard input |
  | `k6 cloud run --local-execution --once <input>` | `api.tar`, or `-` with `api.js` or `api.tar` supplied on standard input |

- **AND** one `api.js` path case for each command places `--once` after the input path
- **THEN** each local execution lists `api` as `1 iterations shared among 1 VUs`
- **AND** each local execution prints `API-RAN` once and reports `1 complete and 0 interrupted iterations`
- **AND** each cloud upload or provisioning request contains one scenario named `api`, with `executor: shared-iterations`, `vus: 1`, `iterations: 1`, and `exec: api`

### Requirement: Non-CLI load cannot replace the scenario

With `--once`, k6 MUST ignore top-level `vus`, `iterations`, `duration`, and `stages` from the script, environment, JSON configuration, and stored archive options. Those values MUST NOT remove, rename, or conflict with an effective scenario.

k6 MUST still parse and decode every source. `--once` MUST NOT hide script, environment, configuration file, or archive errors.

`--once` MUST keep normal precedence between scenario declarations and change only how top-level load fields affect the selected scenario. Without `--once`, k6 MUST keep its existing configuration precedence.

#### Scenario: A named scenario survives load from every non-CLI source

- **GIVEN** each case supplies one complete `api` scenario with the `exec`, env, tags, and `options.browser` from `keep.js`
- **AND** `api` prints those effective values, while a default function prints `FALLBACK-RAN`
- **WHEN** the user runs each input with `--once` under its source case:

  | Scenario source and input | Later or colocated load |
  | --- | --- |
  | Script | top-level `vus: 50` in the same script |
  | Script | each of `K6_VUS=50`, `K6_ITERATIONS=50`, `K6_DURATION=30s`, and `K6_STAGES=30s:50` in separate runs |
  | Explicit JSON config; the script exports `api` but no options | the same config sets top-level `vus` and `iterations` |
  | Default JSON config; the script exports `api` but no options | the same config sets top-level `duration` |
  | Script, while an explicit JSON config declares a different scenario | no top-level load; the script's `api` scenario wins wholesale |
  | Explicit JSON config | the script sets top-level `iterations: 50` |
  | Stored archive options | the same archive stores top-level `vus: 50` |
  | Stored archive options | run the archive with `K6_DURATION=30s` |

- **THEN** every run keeps the `api` scenario and prints its preserved `exec`, env, tags, and `options.browser`
- **AND** every effective scenario has `executor: shared-iterations`, `vus: 1`, and `iterations: 1`
- **AND** the log omits `FALLBACK-RAN`
- **AND** no warning says that top-level load removed `scenarios`
- **AND** when JSON config and the script both declare scenarios, k6 still reports the normal scenario override warning

#### Scenario: A browser scenario survives environment load

- **GIVEN** the `browser.js` fixture from the earlier browser options requirement
- **WHEN** the user runs `K6_ITERATIONS=50 k6 run --once browser.js`
- **THEN** `status=200 title=once fixture` appears once
- **AND** the summary reports a non-zero `browser_data_received` value
- **AND** k6 does not print `browser not found in registry`
- **AND** the run reports `1 complete and 0 interrupted iterations`

#### Scenario: Cloud paths keep the browser scenario

- **GIVEN** `browser.js` and `K6_ITERATIONS=50`
- **WHEN** the user runs it with `k6 cloud run --once`
- **AND** separately with `k6 cloud run --local-execution --once`
- **THEN** the remote uploaded archive contains one scenario named `ui`, with `exec: ui`, `executor: shared-iterations`, `vus: 1`, `iterations: 1`, and `options.browser.type: chromium`
- **AND** the provisioning options for `--local-execution` and the uploaded archive contain the same scenario
- **AND** local execution prints `status=200 title=once fixture` once and reports a non-zero `browser_data_received` value

### Requirement: Cloud archives store the single-run scenario

Archives uploaded by `k6 cloud run --once` and `k6 cloud run --local-execution --once` MUST store the single-run scenario and its preserved configuration. k6 MUST unset their top-level `vus`, `iterations`, `duration`, and `stages`, serialize those fields as `null`, and omit `once` from their options.

The archive MUST run once without `--once` when no CLI, environment, explicit JSON, or default JSON load applies. With `--no-archive-upload`, local execution MUST still use single-run provisioning options, but no archive exists to carry the behavior forward.

#### Scenario: Both cloud paths store a runnable scenario without old load fields

- **GIVEN** `archive.js` declares both top-level `vus: 50` and `duration: '30s'`
- **AND** it declares one `ramping-arrival-rate` scenario named `api`, with `exec: 'api'`, env, tags, `options.browser`, `startRate`, `timeUnit`, one valid stage, `preAllocatedVUs`, and `maxVUs`
- **WHEN** the user runs `k6 cloud run --once archive.js`
- **AND** separately runs `k6 cloud run --local-execution --once archive.js`
- **THEN** the managed create request's `script` part and the local path's presigned upload each contain an archive
- **AND** each archive's `metadata.json.options` has `vus`, `iterations`, `duration`, and `stages` set to `null`
- **AND** each `metadata.json.options` has no `once` key
- **AND** each archive stores exactly one `api` scenario with `exec`, env, tags, and `options.browser` preserved
- **AND** that scenario has `executor: shared-iterations`, `vus: 1`, and `iterations: 1`
- **AND** its `startTime`, `maxDuration`, and `gracefulStop` are `null`, and it omits fields owned only by the old executor
- **AND** the managed path's `POST /cloud/v6/validate_options` body matches the managed archive's `metadata.json.options`
- **AND** plain `k6 run` with either archive and no other load source lists `api` with `maxDuration: 10m0s` and `gracefulStop: 30s`
- **AND** that run executes `api` once and prints no override warning
- **AND** the `--local-execution` provisioning request describes the same scenario with `max_vus: 1` and `total_duration: 630`

#### Scenario: Cloud archives create `default` when the test has no scenario

- **GIVEN** `plain.js` exports a default function that prints `DEFAULT-RAN`, and its options declare top-level `vus: 50` and `duration: '30s'`
- **WHEN** the user runs it separately with `k6 cloud run --once` and `k6 cloud run --local-execution --once`
- **THEN** each uploaded archive has the four top-level load fields set to `null` and one `default` `shared-iterations` scenario with `vus: 1` and `iterations: 1`
- **AND** local execution prints `DEFAULT-RAN` once
- **AND** running either archive with `k6 run`, without load from another source, prints `DEFAULT-RAN` once and reports one complete iteration

#### Scenario: Local execution can omit the archive without changing its run

- **GIVEN** `archive.js`
- **WHEN** the user runs `k6 cloud run --local-execution --no-archive-upload --once archive.js`
- **AND** separately runs it with `K6_NO_ARCHIVE_UPLOAD=true` instead of `--no-archive-upload`
- **THEN** the `start-local-execution` request contains the single-run scenario and `archive_size: null`
- **AND** k6 sends no request to a presigned archive upload URL
- **AND** local execution runs `api` once

### Requirement: Single-run mode validates the result

`--once` MUST validate the new scenario, not discarded load values. k6 MUST still decode source fields, reject unknown executors and scenario fields, and require the selected function to exist.

#### Scenario: Invalid discarded load values do not block the run

- **GIVEN** a decodable `shared-iterations` scenario named `api` with `vus: 5` and `iterations: 1`, which is invalid without `--once` because it has more VUs than iterations
- **WHEN** the user runs it with `k6 run --once`
- **THEN** k6 validates the fresh 1 VU / 1 iteration scenario and runs it once

#### Scenario: Decode and structure errors still fail the run

- **GIVEN** one fixture has `vus: 'not-a-number'` and another has `executor: 'not-an-executor'`
- **WHEN** the user runs each fixture with `k6 run --once`
- **THEN** k6 fails while loading or validating the test as it normally does
- **AND** no scenario iteration runs

#### Scenario: Missing required functions still fail the run

- **GIVEN** these fixtures:

  | Configuration | Exports | Required error text |
  | --- | --- | --- |
  | No scenarios | only `api` | `executor default: function 'default' not found in exports` |
  | One scenario named `ui`, with no `exec` | only `api` | `executor ui: function 'default' not found in exports` |
  | One scenario named `ui`, with `exec: 'target'` | only `api` | `executor ui: function 'target' not found in exports` |
  | One scenario named `ui`, with `exec: ''` | `default` | `exec value cannot be empty` |
  | One scenario whose name is empty | `default` | `scenario name can't be empty` |

- **WHEN** the user runs each fixture with `k6 run --once`
- **THEN** each invocation reports its required text and exits with a non-zero code
- **AND** no scenario iteration runs

### Requirement: Single-run mode does not add browser configuration

`--once` MUST NOT add browser configuration to a scenario that did not declare it.

#### Scenario: A non-browser scenario remains non-browser

- **GIVEN** `non-browser.js` declares one `shared-iterations` scenario named `api` without `options.browser`
- **AND** `api` prints whether `k6/execution` exposes `options.browser` for its scenario
- **WHEN** the user runs `k6 run --once non-browser.js`
- **THEN** the log contains `hasBrowser=false` once
- **AND** the run reports `1 complete and 0 interrupted iterations`

### Requirement: `--once` rejects multiple scenarios

With two or more effective scenarios, k6 MUST run init context to read the options, then reject `--once` before setup, VU initialization, browser launch, or any cloud request.

The error MUST contain `the --once flag can run only a single scenario` and MUST NOT list the declared scenario names.

#### Scenario: Exactly two scenarios fail through every run path

- **GIVEN** `multi.js` prints `INIT-RAN` in init context, declares scenarios named `zulu` and `xray`, defines setup and both scenario functions with distinct markers, and makes `xray` a browser scenario that calls the browser API
- **AND** the user builds `multi.tar` from `multi.js`
- **AND** `K6_BROWSER_EXECUTABLE_PATH` names a nonexistent executable
- **WHEN** the user invokes each applicable path:

  | Path | Input |
  | --- | --- |
  | `k6 run --once` | `multi.js` |
  | `k6 run --once` | `multi.tar` |
  | `k6 cloud run --once` | `multi.js` or `multi.tar` |
  | `k6 cloud run --local-execution --once` | `multi.js` or `multi.tar` |

- **THEN** each invocation reports the required error and exits with a non-zero code
- **AND** the error contains neither `zulu` nor `xray`
- **AND** the log contains `INIT-RAN` but no setup or body markers
- **AND** no browser executable lookup occurs
- **AND** no request reaches a cloud API

#### Scenario: Environment load cannot hide multiple scenarios

- **GIVEN** `multi.js`
- **WHEN** the user runs `K6_ITERATIONS=3 k6 run --once multi.js`
- **THEN** k6 reports the same error for multiple scenarios
- **AND** no scenario iteration runs

### Requirement: `--once` rejects CLI load flags

k6 MUST reject a command line that combines `--once` with `--vus`/`-u`, `--iterations`/`-i`, `--duration`/`-d`, or `--stage`/`-s`.

k6 MUST report one line containing `the --once flag is not compatible with the --<name> flag`, using the conflicting flag's long name. k6 MUST report it before any error that requires loading the test.

An explicitly passed zero MUST still count as a conflict. A malformed flag value may fail earlier during flag parsing.

#### Scenario: Every long and short load flag fails

- **GIVEN** `api.js`, which `--once` accepts on its own
- **WHEN** the user runs `k6 run --once <added flag> api.js` for each row:

  | Added flag | Required text in the error |
  | --- | --- |
  | `--vus 1` or `-u 2` | `the --once flag is not compatible with the --vus flag` |
  | `--iterations 2` or `-i 2` | `the --once flag is not compatible with the --iterations flag` |
  | `--duration 2s` or `-d 2s` | `the --once flag is not compatible with the --duration flag` |
  | `--stage 2s:2` or `-s 2s:2` | `the --once flag is not compatible with the --stage flag` |

- **THEN** every invocation exits with a non-zero code before a scenario iteration runs
- **AND** the error contains the required text

#### Scenario: The direct flag conflict wins before input loading

- **GIVEN** `broken.js` has invalid JavaScript syntax
- **WHEN** the user runs `k6 run --once -i 0 api.js`
- **AND** separately runs `k6 run --once -i 2 broken.js`
- **AND** separately runs `k6 cloud run --once -i 2 broken.js`
- **AND** separately runs `k6 cloud run --local-execution --once -i 2 broken.js`
- **AND** separately runs `k6 run --once -i 2 multi.js`
- **THEN** every error contains `the --once flag is not compatible with the --iterations flag`
- **AND** the `multi.js` case does not report `the --once flag can run only a single scenario`
- **AND** the invalid-script cases do not report the syntax error
- **AND** no request reaches a cloud API

### Requirement: Only bare CLI `--once` activates single-run mode

Single-run mode MUST be off by default. Only bare `--once` on the current invocation may activate it. `--once` MUST accept no value and MUST NOT select a scenario.

An exported `once` option, `K6_ONCE`, `ONCE`, a JSON `once` field, or an archive option named `once` MUST NOT activate single-run mode. Single-run mode MUST NOT become a `Config` or `lib.Options` field. A single-run archive MUST carry the behavior in its scenario configuration, not in a stored flag.

#### Scenario: Single-run mode stays disabled without the flag

- **GIVEN** `api.js`
- **WHEN** the user runs `k6 run api.js`
- **THEN** k6 lists `api` as `4 iterations shared among 4 VUs`
- **AND** the run reports `4 complete and 0 interrupted iterations`

#### Scenario: Normal configuration precedence stays unchanged without `--once`

- **GIVEN** the script-declared `api` scenario and fallback default function from the non-CLI load matrix
- **WHEN** the user runs it without `--once` and with `K6_ITERATIONS=2`
- **THEN** existing k6 behavior applies: the environment load replaces the scenario, the fallback default function runs twice, and k6 reports the existing override warning

#### Scenario: Non-CLI sources cannot activate single-run mode

- **GIVEN** `api.js` normally runs four iterations
- **WHEN** the user runs it without `--once` under each case:

  | Source | Value |
  | --- | --- |
  | Exported script options | `once: true` |
  | Environment | `K6_ONCE=true` |
  | Environment | `ONCE=true` |
  | Explicit JSON config | `{"once": true}` passed with `-c` |
  | Default JSON config | `{"once": true}` at the default config path |
  | Archive metadata | `"once": true` in the stored options |

- **THEN** every run reports four complete iterations
- **AND** no run uses the single-run configuration
- **AND** the exported options case warns that `once` is an unknown exported option

#### Scenario: `--once=api` cannot select a scenario

- **GIVEN** a valid test script
- **WHEN** the user runs `k6 run --once=api script.js`
- **THEN** k6 rejects the invocation as an invalid value for the `once` flag and exits with a non-zero code
- **AND** no scenario iteration runs

### Requirement: Commands that do not run a test reject `--once`

Every command other than `k6 run` and `k6 cloud run` MUST reject `--once` as an unknown flag.

#### Scenario: Archive and cloud upload reject the flag

- **WHEN** the user runs `k6 archive --once api.js`
- **AND** separately runs `k6 cloud upload --once api.js`
- **THEN** each command reports `unknown flag: --once`
- **AND** each command exits with a non-zero code
