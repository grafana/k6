## ADDED Requirements

### Requirement: Bare `--once` enables single-run mode

`--once` MUST be disabled by default. Passing the bare `--once` flag MUST configure one effective `shared-iterations` scenario with `vus: 1` and `iterations: 1`, without requiring a script edit. This proposal defines no value form and no scenario selector for the flag.

The 1 VU / 1 iteration guarantee describes the effective scenario configuration. Setup failure, iteration failure, timeout, interruption, pause, and execution segments MUST keep their normal behavior and can prevent one complete iteration.

`--once` MUST be accepted only by `k6 run` and `k6 cloud run`, including `k6 cloud run --local-execution`, for script, archive, and standard-input sources those commands already support. Every other command MUST reject it as an unknown flag.

#### Scenario: Supported entry points apply the same single-run configuration

- **GIVEN** `api.js` declares one `shared-iterations` scenario named `api`, with `exec: 'api'`, `vus: 4`, and `iterations: 4`, and its `api` function prints `API-RAN`
- **AND** `api.tar` is an archive built from `api.js`
- **WHEN** the user passes bare `--once` through each supported form:

  | Form | Input |
  | --- | --- |
  | `k6 run --once <input>` | `api.js`, `api.tar`, or `-` with either file supplied on standard input |
  | `k6 cloud run --once <input>` | `api.js`, `api.tar`, or `-` with either file supplied on standard input |
  | `k6 cloud run --local-execution --once <input>` | `api.js`, `api.tar`, or `-` with either file supplied on standard input |

- **AND** path-based cases cover `--once` both before and after the input path
- **THEN** each local execution lists `api` as `1 iterations shared among 1 VUs`
- **AND** each local execution prints `API-RAN` once and reports `1 complete and 0 interrupted iterations`
- **AND** each cloud upload or provisioning request contains one scenario named `api`, with `executor: shared-iterations`, `vus: 1`, `iterations: 1`, and `exec: api`

#### Scenario: Single-run mode stays disabled without the flag

- **GIVEN** `api.js`
- **WHEN** the user runs `k6 run api.js`
- **THEN** k6 lists `api` as `4 iterations shared among 4 VUs`
- **AND** the run reports `4 complete and 0 interrupted iterations`

#### Scenario: Commands that do not run a test reject the flag

- **WHEN** the user runs `k6 archive --once api.js`
- **AND** separately runs `k6 cloud upload --once api.js`
- **THEN** each command reports `unknown flag: --once`
- **AND** each command exits with a non-zero code

#### Scenario: `--once=api` is not a scenario selector

- **GIVEN** a valid test script
- **WHEN** the user runs `k6 run --once=api script.js`
- **THEN** k6 rejects the invocation as an invalid value for the `once` flag and exits with a non-zero code
- **AND** no scenario iteration runs

### Requirement: The effective scenario is a fresh shared-iterations configuration

Single-run mode MUST build a fresh `shared-iterations` scenario. It MUST preserve only the original scenario name, `exec`, `env`, `tags`, and `options`, then set `vus: 1` and `iterations: 1`.

It MUST discard the original `executor`, `vus`, `iterations`, `duration`, `startTime`, `maxDuration`, `timeUnit`, `stages`, `rate`, `startRate`, `gracefulStop`, `gracefulRampDown`, `preAllocatedVUs`, `startVUs`, `maxVUs`, and any other unpreserved scenario field. Discarded fields MUST NOT be copied and then deleted from a known list, because current and future executors can add fields.

The fresh scenario MUST leave `maxDuration` and `gracefulStop` unset. They serialize as `null`, while `shared-iterations` applies its normal 10 minute and 30 second runtime defaults.

#### Scenario: Every built-in executor becomes the same single-run scenario

- **GIVEN** six scripts whose one `api` scenario uses the following valid configurations and whose `api` function prints its script-visible scenario configuration from `k6/execution`:

  | Executor | Declared load |
  | --- | --- |
  | `shared-iterations` | `vus: 8`, `iterations: 20`, `maxDuration: '1h'`, `gracefulStop: '5m'` |
  | `per-vu-iterations` | `vus: 8`, `iterations: 20`, `maxDuration: '1h'` |
  | `constant-vus` | `vus: 8`, `duration: '30s'`, `startTime: '30s'` |
  | `ramping-vus` | `startVUs: 8`, one stage, `gracefulRampDown: '10s'` |
  | `constant-arrival-rate` | `rate: 100`, `timeUnit: '1s'`, `duration: '30s'`, `preAllocatedVUs: 10`, `maxVUs: 20` |
  | `ramping-arrival-rate` | `startRate: 10`, `timeUnit: '1s'`, one stage, `preAllocatedVUs: 10`, `maxVUs: 20` |

- **WHEN** the user runs each script with `k6 run --once`
- **THEN** each script-visible scenario is named `api`, contains `executor: shared-iterations`, `vus: 1`, and `iterations: 1`, and has `startTime`, `maxDuration`, and `gracefulStop` set to `null`
- **AND** each pre-run line shows `maxDuration: 10m0s` and `gracefulStop: 30s`, the run starts without the declared delay, and one iteration completes
- **AND** no scenario contains a field that `shared-iterations` does not own or an original load-shaping value

#### Scenario: Semantically invalid discarded load does not block the run

- **GIVEN** a decodable `shared-iterations` scenario named `api` with `vus: 5` and `iterations: 1`, which is invalid without `--once` because it has more VUs than iterations
- **WHEN** the user runs it with `k6 run --once`
- **THEN** k6 validates the fresh 1 VU / 1 iteration scenario and runs it once
- **BUT WHEN** a field cannot be decoded, the executor type is missing or unknown, the scenario contains an unknown field, or its preserved `exec` names a function that is not exported
- **THEN** k6 fails while loading or validating the test as it normally does

### Requirement: The scenario keeps its identity and non-load configuration

Single-run mode MUST preserve the running scenario's name, `exec`, `env`, `tags`, and complete `options` block. Samples emitted during the scenario iteration MUST use the preserved scenario name and tags. Samples emitted by setup or teardown are outside that claim.

#### Scenario: Name, exec, env, tags, options, and the script-visible configuration survive

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
- **AND** `DEFAULT-RAN` is absent
- **AND** every `type: Point` entry emitted by the scenario iteration in `out.json` has `data.tags.scenario: api` and `data.tags.team: core`

### Requirement: A test without scenarios runs its default function

When the effective configuration contains no scenario and the script exports `default`, single-run mode MUST create a scenario named `default` with the single-run configuration. It MUST clear the top-level `vus`, `iterations`, `duration`, and `stages` fields before scenario derivation.

#### Scenario: Every no-scenario form runs the default function once

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

### Requirement: A single declared scenario decides which function runs

When the effective configuration contains one scenario, single-run mode MUST keep that scenario. A preserved non-empty `exec` value MUST select that exported function. A missing `exec` value or `exec: 'default'` MUST select the default function. The selected function MUST exist.

#### Scenario: Missing exec and explicit default exec behave the same

- **GIVEN** two versions of a script, each with one scenario named `ui` and `env: { URL: 'https://example.com' }`, declaring either no `exec` or `exec: 'default'`
- **AND** each default function prints `DEFAULT-RAN url=<URL> exec=<effective exec>` from `k6/execution`
- **WHEN** the user runs each script with `k6 run --once`
- **THEN** each run lists a scenario named `ui`, not `default`
- **AND** the missing-exec case prints `DEFAULT-RAN url=https://example.com exec=null` once
- **AND** the explicit case prints `DEFAULT-RAN url=https://example.com exec=default` once

#### Scenario: A required function that is not exported still fails

- **GIVEN** these fixtures:

  | Configuration | Exports | Required error text |
  | --- | --- | --- |
  | No scenarios | only `api` | `executor default: function 'default' not found in exports` |
  | One scenario named `ui`, with no `exec` | only `api` | `executor ui: function 'default' not found in exports` |
  | One scenario named `ui`, with `exec: 'target'` | only `api` | `executor ui: function 'target' not found in exports` |
  | One scenario named `ui`, with `exec: ''` | `default` | `exec value cannot be empty` |
  | One scenario whose name is empty | `default` | `scenario name can't be empty` |
  | Empty script | none | `no exported functions in script` |

- **WHEN** the user runs each fixture with `k6 run --once`
- **THEN** each invocation reports its required text and exits with a non-zero code
- **AND** no scenario iteration runs

### Requirement: Non-CLI load cannot remove the scenario

Single-run mode MUST ignore top-level `vus`, `iterations`, `duration`, and `stages` from the script, environment, JSON configuration, and stored archive options. Those values MUST NOT remove or rename an effective scenario and MUST NOT cause a conflict with it.

The source still MUST be syntactically valid. Single-run mode does not suppress script, environment, configuration-file, or archive decoding errors.

Scenario declarations from different sources MUST keep normal scenario-to-scenario precedence. The presence of `--once` changes only how top-level load fields affect those scenarios. Without `--once`, existing configuration precedence MUST remain unchanged.

#### Scenario: A named scenario survives load from every non-CLI source

- **GIVEN** each case supplies one complete `api` scenario with the `exec`, env, tags, and `options.browser` from `keep.js`; `api` prints those effective values, and a default function prints `FALLBACK-RAN`
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
  | Stored archive options | `K6_DURATION=30s` is set when running the archive |

- **THEN** every run keeps the `api` scenario and prints its preserved `exec`, env, tags, and `options.browser`
- **AND** every effective scenario has `executor: shared-iterations`, `vus: 1`, and `iterations: 1`
- **AND** `FALLBACK-RAN` is absent
- **AND** no warning says that top-level load removed `scenarios`; the normal scenario-to-scenario override warning remains in the config-versus-script case

#### Scenario: Normal configuration precedence is unchanged without `--once`

- **GIVEN** the script-declared `api` scenario and fallback default function from the matrix above
- **WHEN** the user runs it without `--once` and with `K6_ITERATIONS=2`
- **THEN** existing k6 behavior applies: the environment load replaces the scenario, the fallback default function runs twice, and k6 reports the existing override warning

### Requirement: Two or more effective scenarios are rejected

When the effective configuration contains two or more scenarios, single-run mode MUST fail before setup, VU initialization, browser launch, or any cloud request. Init context still runs because k6 evaluates it to read the options.

The error MUST contain `the --once flag can run only a single scenario` and MUST NOT list the declared scenario names.

#### Scenario: The boundary case of exactly two scenarios is rejected through every run path

- **GIVEN** `multi.js` prints `INIT-RAN` in init context, declares scenarios named `zulu` and `xray`, defines setup and both scenario functions with distinct markers, and makes `xray` a browser scenario that calls the browser API
- **AND** `multi.tar` is an archive built from `multi.js`
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
- **AND** `INIT-RAN` is present, while setup and body markers are absent
- **AND** no browser executable lookup occurs
- **AND** no request reaches a cloud API

#### Scenario: Higher-tier load cannot hide multiple scenarios

- **GIVEN** `multi.js`
- **WHEN** the user runs `K6_ITERATIONS=3 k6 run --once multi.js`
- **THEN** k6 reports the same multi-scenario error
- **AND** no scenario iteration runs

### Requirement: CLI load flags conflict with `--once`

An enabled `--once` MUST reject `--vus`/`-u`, `--iterations`/`-i`, `--duration`/`-d`, and `--stage`/`-s` when the user passes both on the same command line. The single-line error MUST contain `the --once flag is not compatible with the --<name> flag`, using the conflicting flag's long name. This direct command-line error MUST take precedence over errors that require loading the test. An explicitly passed zero still counts as a conflict; a malformed flag value can fail earlier during flag parsing.

#### Scenario: Every long and short load flag is rejected

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
- **AND** the multi-scenario case does not report the multi-scenario failure
- **AND** the invalid-script cases do not report the syntax error
- **AND** no request reaches a cloud API

### Requirement: Browser configuration survives single-run mode

Single-run mode MUST preserve `options.browser` on a declared scenario and MUST keep the scenario name used to find that configuration. It MUST NOT add browser configuration to a scenario that did not declare it.

#### Scenario: A browser scenario survives an environment load override

- **GIVEN** a controlled local HTTP fixture that responds with HTTP 200 and the title `once fixture`
- **AND** `browser.js` declares one `shared-iterations` scenario named `ui`, with `exec: 'ui'`, 10 VUs, 10 iterations, and `options.browser.type: chromium`
- **AND** `ui` opens the fixture and prints its response status and page title
- **WHEN** the user runs `K6_ITERATIONS=50 k6 run --once browser.js`
- **THEN** `status=200 title=once fixture` appears once
- **AND** the summary reports a non-zero `browser_data_received` value
- **AND** `browser not found in registry` is absent
- **AND** the run reports `1 complete and 0 interrupted iterations`

#### Scenario: Cloud paths receive the preserved browser scenario

- **GIVEN** `browser.js` and `K6_ITERATIONS=50`
- **WHEN** the user runs it with `k6 cloud run --once`
- **AND** separately with `k6 cloud run --local-execution --once`
- **THEN** the remote uploaded archive contains one scenario named `ui`, with `exec: ui`, `executor: shared-iterations`, `vus: 1`, `iterations: 1`, and `options.browser.type: chromium`
- **AND** the local-execution provisioning options and uploaded archive contain the same scenario
- **AND** local execution prints `status=200 title=once fixture` once and reports a non-zero `browser_data_received` value

#### Scenario: Single-run mode does not invent browser options

- **GIVEN** one test with a scenario named `ui` that calls the browser API without declaring `options.browser`
- **AND** another test that declares no scenarios and calls the browser API from `default`
- **AND** `K6_BROWSER_EXECUTABLE_PATH` names a nonexistent executable
- **WHEN** the user runs each test with `k6 run --once`
- **THEN** each run reports `browser not found in registry`
- **AND** neither run reports an error from looking up or launching the nonexistent executable
- **AND** each run exits with code 0, unchanged from the same test without `--once`

### Requirement: Single-run mode is CLI-only

Only the bare flag on the current invocation may enable single-run mode. An exported script option, `K6_ONCE`, `ONCE`, a JSON configuration field, or an archive option named `once` MUST be inert. Single-run mode MUST NOT become a `Config` or `lib.Options` field, and archives created for a single run MUST carry the behavior through their scenario configuration instead of a stored flag.

#### Scenario: Non-CLI sources cannot enable single-run mode

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
- **AND** the script-options case warns that `once` is an unknown exported option

### Requirement: Setup and teardown keep normal behavior

Single-run mode MUST preserve setup and teardown execution, their data flow, their timeouts, and the existing options that skip them.

#### Scenario: Setup data reaches the selected function and teardown

- **GIVEN** `lifecycle.js` declares one `shared-iterations` scenario named `api`, with `exec: 'api'`, 50 VUs, and 50 iterations
- **AND** setup prints `SETUP-RAN` and returns `{ token: 'abc' }`
- **AND** `api(data)` prints `API-RAN token=abc`
- **AND** teardown prints `TEARDOWN-RAN token=abc`
- **AND** an exported default function prints `DEFAULT-RAN`
- **WHEN** the user runs `k6 run --once lifecycle.js`
- **THEN** the log shows `SETUP-RAN`, `API-RAN token=abc`, and `TEARDOWN-RAN token=abc` once each and in that order
- **AND** `DEFAULT-RAN` is absent
- **AND** the run reports one complete iteration

#### Scenario: Existing skip controls still skip their lifecycle functions

- **GIVEN** a lifecycle fixture whose VU function and teardown do not read setup data
- **WHEN** the user runs it with `k6 run --once` under each case:

  | Source | Skip values |
  | --- | --- |
  | Command line | `--no-setup --no-teardown` |
  | Environment | `K6_NO_SETUP=true K6_NO_TEARDOWN=true` |
  | Script options | `noSetup: true, noTeardown: true` |
  | JSON config | `noSetup: true, noTeardown: true` |

- **THEN** neither lifecycle marker appears
- **AND** the selected VU function runs once, no script exception is reported, and the process exits successfully

### Requirement: Cloud archives carry the single-run scenario

An archive uploaded by `k6 cloud run --once` or `k6 cloud run --local-execution --once` MUST store the single-run scenario and its preserved configuration. Its top-level `vus`, `iterations`, `duration`, and `stages` fields MUST be unset and serialize as `null`. Its options MUST NOT contain a `once` key.

Running the captured archive without `--once` MUST run once when no command-line, environment, explicit JSON config, or default JSON config supplies load. With `--no-archive-upload`, local execution MUST still use the single-run provisioning options, but no archive exists to carry forward.

#### Scenario: Both cloud archive paths store a clean, runnable scenario

- **GIVEN** `archive.js` declares both top-level `vus: 50` and `duration: '30s'`
- **AND** it declares one `ramping-arrival-rate` scenario named `api`, with `exec: 'api'`, env, tags, `options.browser`, `startRate`, `timeUnit`, one valid stage, `preAllocatedVUs`, and `maxVUs`
- **WHEN** the user runs `k6 cloud run --once archive.js`
- **AND** separately runs `k6 cloud run --local-execution --once archive.js`
- **THEN** the managed create request's `script` part and the local path's presigned upload each contain an archive whose `metadata.json.options` has `vus`, `iterations`, `duration`, and `stages` set to `null`
- **AND** each `metadata.json.options` has no `once` key
- **AND** each archive stores exactly one `api` scenario with `exec`, env, tags, and `options.browser` preserved
- **AND** that scenario has `executor: shared-iterations`, `vus: 1`, and `iterations: 1`; `startTime`, `maxDuration`, and `gracefulStop` are `null`, and fields owned only by the old executor are absent
- **AND** the managed path's `POST /cloud/v6/validate_options` body contains the same clean options
- **AND** running either archive with `k6 run`, without `--once` or load from another source, lists `api` with `maxDuration: 10m0s` and `gracefulStop: 30s`, runs it once, and prints no override warning
- **AND** the local-execution provisioning request describes the same scenario with `max_vus: 1` and `total_duration: 630`

#### Scenario: Cloud archives store the synthesized default scenario

- **GIVEN** `plain.js` exports a default function that prints `DEFAULT-RAN`, and its options declare top-level `vus: 50` and `duration: '30s'`
- **WHEN** the user runs it separately with `k6 cloud run --once` and `k6 cloud run --local-execution --once`
- **THEN** each uploaded archive has the four top-level load fields set to `null` and one `default` `shared-iterations` scenario with `vus: 1` and `iterations: 1`
- **AND** local execution prints `DEFAULT-RAN` once
- **AND** running either archive with `k6 run`, without load from another source, prints `DEFAULT-RAN` once and reports one complete iteration

#### Scenario: Local execution can omit the archive without changing its run

- **GIVEN** `archive.js`
- **WHEN** the user runs `k6 cloud run --local-execution --no-archive-upload --once archive.js`
- **AND** separately runs it with `K6_NO_ARCHIVE_UPLOAD=true` instead of `--no-archive-upload`
- **THEN** the start-local-execution request contains the single-run scenario and `archive_size: null`
- **AND** no request is made to a presigned archive-upload URL
- **AND** local execution runs `api` once

### Requirement: Non-scenario options keep normal behavior

Apart from replacing the effective scenario and clearing the four top-level load fields, single-run mode MUST leave every consolidated option unchanged. Thresholds MUST still decide the process exit code. Pause and execution-segment options MUST still act on the resulting single-run scenario.

#### Scenario: A failing threshold still fails the single run

- **GIVEN** a test with one 50 VU / 50 iteration scenario, a check that always fails, and `thresholds: { checks: ['rate==1.0'] }`
- **WHEN** the user runs `k6 run --once threshold.js`
- **THEN** the test performs one iteration
- **AND** k6 reports that the `checks` threshold was crossed
- **AND** the process exits with the threshold-failure code

#### Scenario: Pause holds the iteration until the user resumes it

- **GIVEN** `api.js` and a free REST API address
- **WHEN** the user runs `k6 run --once --paused --address <address> api.js`
- **THEN** `GET /v1/status` reports `paused: true` and status `4`, while `API-RAN` is absent
- **AND WHEN** the user sends `PATCH /v1/status` with `{"data":{"type":"status","id":"default","attributes":{"paused":false}}}` and receives HTTP 200
- **THEN** `API-RAN` appears once and the run reports one complete iteration

#### Scenario: Execution segments retain their existing scaling behavior

- **GIVEN** `api.js`
- **WHEN** the user runs `k6 run --once --execution-segment 0:1/2 --execution-segment-sequence 0,1/2,1 api.js`
- **THEN** `API-RAN` appears once and the run reports one complete iteration
- **BUT WHEN** the user runs `k6 run --once --execution-segment 1/2:1 --execution-segment-sequence 0,1/2,1 api.js`
- **THEN** `API-RAN` is absent and k6 reports zero VUs and zero complete iterations
- **AND** the process exits with code 0, even if k6 also logs its existing `failed to handle the end-of-test summary` error
