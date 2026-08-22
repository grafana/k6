## ADDED Requirements

### Requirement: Bare `--once` runs one VU for one iteration

Passing bare `--once` MUST configure exactly one effective `shared-iterations` scenario with `vus: 1` and `iterations: 1`. It MUST work without a script change.

#### Scenario: A declared scenario runs once

- **GIVEN** `api.js` declares one `shared-iterations` scenario named `api`, with `exec: 'api'`, `vus: 4`, and `iterations: 4`, and its `api` function prints `API-RAN`
- **WHEN** the user runs `k6 run --once api.js`
- **THEN** the execution banner lists `api` as `1 iterations shared among 1 VUs`
- **AND** `API-RAN` appears once
- **AND** the final progress line reports `1 complete and 0 interrupted iterations`

### Requirement: `--once` creates a new `shared-iterations` scenario

`--once` MUST build a new scenario as follows:

| Action | Scenario data |
| --- | --- |
| Preserve | scenario name, `exec`, `env`, `tags`, and the complete `options` block |
| Set | `executor: shared-iterations`, `vus: 1`, and `iterations: 1` |
| Discard | every other field from the original scenario |

Discarded fields include the original `executor`, `vus`, `iterations`, `duration`, `startTime`, `maxDuration`, `timeUnit`, `stages`, `rate`, `startRate`, `gracefulStop`, `gracefulRampDown`, `preAllocatedVUs`, `startVUs`, and `maxVUs`.

`--once` MUST leave `maxDuration` and `gracefulStop` unset. k6 MUST serialize them as `null` and apply the `shared-iterations` defaults at runtime: 10 minutes and 30 seconds.

Iteration samples MUST use the preserved scenario tags. When the `scenario` system tag is enabled, they MUST also use the preserved scenario name.

#### Scenario: Every executor becomes a scenario with one VU and one iteration

- **GIVEN** six scripts that each declare one scenario named `api`
- **AND** each `api` function prints its effective scenario from `k6/execution`
- **AND** the declared scenarios use these valid configurations:

  | Executor | Declared load |
  | --- | --- |
  | `shared-iterations` | `vus: 8`, `iterations: 20`, `maxDuration: '1h'`, `gracefulStop: '5m'` |
  | `per-vu-iterations` | `vus: 8`, `iterations: 20`, `maxDuration: '1h'` |
  | `constant-vus` | `vus: 8`, `duration: '30s'`, `startTime: '30s'` |
  | `ramping-vus` | `startVUs: 8`, `stages: [{ duration: '30s', target: 16 }]`, `gracefulRampDown: '10s'` |
  | `constant-arrival-rate` | `rate: 100`, `timeUnit: '1s'`, `duration: '30s'`, `preAllocatedVUs: 10`, `maxVUs: 20` |
  | `ramping-arrival-rate` | `startRate: 10`, `timeUnit: '1s'`, `stages: [{ duration: '30s', target: 20 }]`, `preAllocatedVUs: 10`, `maxVUs: 20` |

- **WHEN** the user runs each script with `k6 run --once`
- **THEN** each `api` function prints a scenario named `api` with `executor: shared-iterations`, `vus: 1`, and `iterations: 1`
- **AND** `startTime`, `maxDuration`, and `gracefulStop` are `null`
- **AND** each execution banner shows `maxDuration: 10m0s` and `gracefulStop: 30s`
- **AND** each run starts without the declared delay and completes one iteration
- **AND** each printed scenario omits `duration`, `timeUnit`, `stages`, `rate`, `startRate`, `gracefulRampDown`, `preAllocatedVUs`, `startVUs`, and `maxVUs`

#### Scenario: `--once` keeps the scenario identity and options

- **GIVEN** `keep.js`:

  ```js
  import exec from 'k6/execution';
  import { Counter } from 'k6/metrics';

  const kept = new Counter('kept');

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
    kept.add(1);
    console.log(`name=${exec.scenario.name} exec=${scenario.exec} executor=${scenario.executor} vus=${scenario.vus} iterations=${scenario.iterations} url=${__ENV.URL} browser=${scenario.options.browser.type}`);
  }

  export default function () { console.log('DEFAULT-RAN'); }
  ```

- **WHEN** the user runs `k6 run --once --out json=out.json keep.js`
- **THEN** the log contains `name=api exec=api executor=shared-iterations vus=1 iterations=1 url=https://example.com browser=chromium` once
- **AND** the log omits `DEFAULT-RAN`
- **AND** `out.json` contains exactly one `type: Point` entry for the `kept` metric, with `data.tags.scenario: api` and `data.tags.team: core`

### Requirement: One declared scenario chooses the function

With one effective scenario, `--once` MUST run it. When `exec` has a value, it selects that export. A missing `exec` or `exec: 'default'` selects `default`.

#### Scenario: Missing exec and explicit default exec behave the same

- **GIVEN** two versions of a script, each with one scenario named `ui` and `env: { URL: 'https://example.com' }`, declaring either no `exec` or `exec: 'default'`
- **AND** each default function prints `DEFAULT-RAN url=<URL> exec=<effective exec>` from `k6/execution`
- **WHEN** the user runs each script with `k6 run --once`
- **THEN** each run lists a scenario named `ui`, not `default`
- **AND** the case without `exec` prints `DEFAULT-RAN url=https://example.com exec=null` once
- **AND** the explicit case prints `DEFAULT-RAN url=https://example.com exec=default` once

### Requirement: `--once` keeps browser options

`--once` MUST preserve a declared scenario's name and `options.browser` block.

#### Scenario: A browser scenario keeps its configuration and runs once

- **GIVEN** a controlled local HTTP fixture that responds with HTTP 200 and the title `once fixture`
- **AND** `browser.js` declares one `shared-iterations` scenario named `ui`, with `exec: 'ui'`, 10 VUs, 10 iterations, and `options.browser.type: chromium`
- **AND** `ui` opens the fixture and prints its response status and page title
- **WHEN** the user runs `k6 run --once browser.js`
- **THEN** `status=200 title=once fixture` appears once
- **AND** the summary reports `browser_data_received` greater than zero
- **AND** the run reports `1 complete and 0 interrupted iterations`

### Requirement: A test without scenarios runs `default`

If the effective configuration has no scenario and the script exports `default`, `--once` MUST create a scenario named `default` with one VU and one iteration. It MUST also clear `vus`, `iterations`, `duration`, and `stages` outside `scenarios`.

#### Scenario: Every form without scenarios runs `default` once

- **GIVEN** scripts that export a `default` function printing `DEFAULT-RAN` and use one of these option forms:

  | Form | Options |
  | --- | --- |
  | No options | no `options` export |
  | Empty scenarios | `{ scenarios: {} }` |
  | Null scenarios | `{ scenarios: null }` |
  | VU shortcut | `{ vus: 8 }` |
  | Duration shortcut | `{ vus: 8, duration: '30s' }` |
  | Iteration shortcut | `{ vus: 8, iterations: 20 }` |
  | Stage shortcut | `{ stages: [{ duration: '30s', target: 8 }] }` |

- **WHEN** the user runs each script with `k6 run --once`
- **THEN** k6 lists `default` as `1 iterations shared among 1 VUs`
- **AND** `DEFAULT-RAN` appears once
- **AND** the run reports `1 complete and 0 interrupted iterations`

### Requirement: `--once` preserves setup and teardown

`--once` MUST run setup before the selected function, pass its return value to that function and teardown, and run teardown afterward. `--no-setup`, `--no-teardown`, `noSetup`, and `noTeardown` MUST still skip them.

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

#### Scenario: CLI and JSON options still skip setup and teardown

- **GIVEN** a lifecycle fixture where setup prints `SETUP-RAN`, the selected VU function prints `API-RAN`, and teardown prints `TEARDOWN-RAN`
- **AND** its selected VU function and teardown do not read setup data
- **WHEN** the user runs it with `k6 run --once` under each case:

  | Source | Skip value | Required markers | Omitted marker |
  | --- | --- | --- | --- |
  | Command line | `--no-setup` | `API-RAN`, `TEARDOWN-RAN` | `SETUP-RAN` |
  | Command line | `--no-teardown` | `SETUP-RAN`, `API-RAN` | `TEARDOWN-RAN` |
  | JSON config | `noSetup: true` | `API-RAN`, `TEARDOWN-RAN` | `SETUP-RAN` |
  | JSON config | `noTeardown: true` | `SETUP-RAN`, `API-RAN` | `TEARDOWN-RAN` |

- **THEN** the log contains each required marker once and omits the row's omitted marker
- **AND** k6 reports no script exception and exits with code 0

### Requirement: Other options still control the run

k6 MUST apply `--once` after it combines options from the script, environment, JSON config, archive, and CLI. It MUST replace only the effective scenario and clear only `vus`, `iterations`, `duration`, and `stages` outside `scenarios`. It MUST leave every other option unchanged. Thresholds still determine the exit code. Pause and execution segment options still apply.

`--once` configures one iteration but does not bypass a threshold, pause, or execution segment.

#### Scenario: A failing threshold still fails the single run

- **GIVEN** `threshold.js` has one 50 VU / 50 iteration scenario, a check that always fails, and `thresholds: { checks: ['rate==1.0'] }`
- **WHEN** the user runs `k6 run --once threshold.js`
- **THEN** the test performs one iteration
- **AND** k6 reports that `checks` crossed its threshold
- **AND** the process exits with code 99

#### Scenario: Pause holds the iteration until the user resumes it

- **GIVEN** `api.js` and a free REST API address
- **WHEN** the user runs `k6 run --once --paused --address <address> api.js`
- **THEN** `GET /v1/status` reports `paused: true` and status `4`, while the log omits `API-RAN`
- **AND WHEN** the user sends `PATCH /v1/status` with `{"data":{"type":"status","id":"default","attributes":{"paused":false}}}` and receives HTTP 200
- **THEN** `API-RAN` appears once and the run reports one complete iteration

#### Scenario: An execution segment can reduce the run to zero iterations

- **GIVEN** `api.js`
- **WHEN** the user runs `k6 run --once --execution-segment <value> --execution-segment-sequence 0,1/2,1 api.js` for each case:

  | `--execution-segment` | Expected result |
  | --- | --- |
  | `0:1/2` | `API-RAN` appears once and the run reports one complete iteration |
  | `1/2:1` | `API-RAN` does not appear, the run reports zero VUs and zero complete iterations, and the process exits with code 0 |

- **THEN** each run matches its expected result

### Requirement: Script files, archives, and standard input work with `--once`

`k6 run`, `k6 cloud run`, and `k6 cloud run --local-execution` MUST accept `--once` with script files, archives, and scripts or archives supplied on standard input.

`api.js` declares one `shared-iterations` scenario named `api` with `exec: 'api'`, four VUs, four iterations, and an `api` function that prints `API-RAN`. `api.tar` is an archive built from `api.js`.

#### Scenario: Local runs accept archives and standard input

- **GIVEN** `api.js` and `api.tar`
- **WHEN** the user runs `k6 run --once` with `api.tar`, `api.js` on standard input, and `api.tar` on standard input in separate invocations
- **THEN** each execution banner lists `api` as `1 iterations shared among 1 VUs`
- **AND** each run prints `API-RAN` once and reports `1 complete and 0 interrupted iterations`

#### Scenario: Remote cloud runs accept archives and standard input

- **GIVEN** `api.js` and `api.tar`
- **WHEN** the user runs `k6 cloud run --once` with `api.tar`, `api.js` on standard input, and `api.tar` on standard input in separate invocations
- **THEN** each cloud upload contains one scenario named `api`, with `executor: shared-iterations`, `vus: 1`, `iterations: 1`, and `exec: api`

#### Scenario: Local cloud runs accept archives and standard input

- **GIVEN** `api.js` and `api.tar`
- **WHEN** the user runs `k6 cloud run --local-execution --once` with `api.tar`, `api.js` on standard input, and `api.tar` on standard input in separate invocations
- **THEN** each provisioning request contains one scenario named `api`, with `executor: shared-iterations`, `vus: 1`, `iterations: 1`, and `exec: api`
- **AND** each execution banner lists `api` as `1 iterations shared among 1 VUs`
- **AND** each run prints `API-RAN` once and reports `1 complete and 0 interrupted iterations`

### Requirement: Load from other sources cannot replace the scenario

With `--once`, k6 MUST ignore `vus`, `iterations`, `duration`, and `stages` outside `scenarios`. This applies when they come from the script, environment, JSON config, or archive. Those values MUST NOT remove, rename, or conflict with the effective scenario.

When both the script and a JSON config declare scenarios, k6 MUST keep the script scenario. `--once` changes only how load fields outside `scenarios` affect that scenario.

#### Scenario: `--once` keeps the selected scenario across option sources

- **GIVEN** each case supplies one `api` scenario with `exec: 'api'`, `env.URL: 'https://example.com'`, `tags.team: 'core'`, and `options.browser.type: chromium`
- **AND** `api` prints its scenario values as `API-RAN executor=<executor> vus=<vus> iterations=<iterations> exec=<exec> url=<URL> team=<team> browser=<browser>`, while a default function prints `FALLBACK-RAN`
- **WHEN** the user runs each input with `--once` under its source case:

  | Scenario source | Other load configuration |
  | --- | --- |
  | Script | `vus: 50` outside `scenarios` in the same script |
  | Script | each of `K6_VUS=50`, `K6_ITERATIONS=50`, `K6_DURATION=30s`, and `K6_STAGES=30s:50` in separate runs |
  | Explicit JSON config; the script exports `api` but no options | the same config sets `vus` and `iterations` outside `scenarios` |
  | Default JSON config; the script exports `api` but no options | the same config sets `duration` outside `scenarios` |
  | Script, while an explicit JSON config declares a different scenario | the script's `api` scenario wins; neither source sets load outside `scenarios` |
  | Explicit JSON config | the script sets `iterations: 50` outside `scenarios` |
  | Stored archive options | the same archive stores `vus: 50` outside `scenarios` |
  | Stored archive options | run the archive with `K6_DURATION=30s` |

- **THEN** every run prints `API-RAN executor=shared-iterations vus=1 iterations=1 exec=api url=https://example.com team=core browser=chromium` once
- **AND** the log omits `FALLBACK-RAN`

#### Scenario: `K6_ITERATIONS` does not remove the browser scenario

- **GIVEN** `browser.js` and its controlled local HTTP fixture
- **WHEN** the user runs `K6_ITERATIONS=50 k6 run --once browser.js`
- **THEN** `status=200 title=once fixture` appears once
- **AND** the summary reports `browser_data_received` greater than zero
- **AND** the run reports `1 complete and 0 interrupted iterations`

#### Scenario: Remote cloud execution keeps the browser scenario

- **GIVEN** `browser.js` and `K6_ITERATIONS=50`
- **WHEN** the user runs `k6 cloud run --once browser.js`
- **THEN** the archive uploaded for remote execution contains one scenario named `ui`, with `exec: ui`, `executor: shared-iterations`, `vus: 1`, `iterations: 1`, and `options.browser.type: chromium`

#### Scenario: Local cloud execution keeps and runs the browser scenario

- **GIVEN** `browser.js`, its controlled local HTTP fixture, and `K6_ITERATIONS=50`
- **WHEN** the user runs `k6 cloud run --local-execution --once browser.js`
- **THEN** the uploaded archive and provisioning request contain one scenario named `ui`, with `exec: ui`, `executor: shared-iterations`, `vus: 1`, `iterations: 1`, and `options.browser.type: chromium`
- **AND** local execution prints `status=200 title=once fixture` once and reports `browser_data_received` greater than zero

### Requirement: Cloud archives store the scenario created by `--once`

Archives uploaded by `k6 cloud run --once` and `k6 cloud run --local-execution --once` MUST store the scenario created by `--once` and its preserved configuration.

k6 MUST unset `vus`, `iterations`, `duration`, and `stages` outside `scenarios`, serialize them as `null`, and omit `once` from the archive options.

The archive MUST run once without `--once` when no other source supplies load. With `--no-archive-upload`, local execution MUST still provision one VU and one iteration.

The local execution provisioning request MUST set `max_vus: 1` and `total_duration: 630`. The 630 seconds include the 10 minute max duration and 30 second graceful stop.

`archive.js` exports an `api` function that prints `API-RAN` and declares these options:

| Location | Configuration |
| --- | --- |
| Outside `scenarios` | `vus: 50`, `duration: '30s'` |
| Scenario `api` | `executor: 'ramping-arrival-rate'`, `exec: 'api'`, `env.URL: 'https://example.com'`, `tags.team: 'core'`, `options.browser.type: chromium`, `startRate: 10`, `timeUnit: '1s'`, `stages: [{ duration: '30s', target: 20 }]`, `preAllocatedVUs: 10`, `maxVUs: 20` |

#### Scenario: Both cloud paths upload the replacement scenario

- **GIVEN** `archive.js`
- **WHEN** the user runs each cloud path:

  | Command | Archive location |
  | --- | --- |
  | `k6 cloud run --once archive.js` | the request's `script` field |
  | `k6 cloud run --local-execution --once archive.js` | the presigned upload URL |

- **THEN** each archive's `metadata.json.options` has `vus`, `iterations`, `duration`, and `stages` set to `null`
- **AND** each `metadata.json.options` has no `once` key
- **AND** each archive stores exactly one `api` scenario with `exec: api`, `env.URL: https://example.com`, `tags.team: core`, and `options.browser.type: chromium`
- **AND** that scenario has `executor: shared-iterations`, `vus: 1`, and `iterations: 1`
- **AND** its `startTime`, `maxDuration`, and `gracefulStop` are `null`, and it omits settings used only by the original executor
- **AND** the `k6 cloud run` request to `POST /cloud/v6/validate_options` matches that archive's `metadata.json.options`

#### Scenario: A cloud archive runs once without the flag

- **GIVEN** the archives uploaded from `archive.js` by both cloud paths
- **WHEN** the user runs each archive with plain `k6 run` and no load from another source
- **THEN** each execution banner lists `api` with `maxDuration: 10m0s` and `gracefulStop: 30s`
- **AND** each run prints `API-RAN` once and reports one complete iteration

#### Scenario: Local cloud execution provisions one VU and one iteration

- **GIVEN** `archive.js`
- **WHEN** the user runs `k6 cloud run --local-execution --once archive.js`
- **THEN** the provisioning request contains the replacement scenario with `max_vus: 1` and `total_duration: 630`

#### Scenario: Cloud archives create `default` when the test has no scenario

- **GIVEN** `plain.js` exports a default function, and its options declare `vus: 50` and `duration: '30s'` outside `scenarios`
- **WHEN** the user runs it separately with `k6 cloud run --once` and `k6 cloud run --local-execution --once`
- **THEN** each uploaded archive sets the four load fields outside `scenarios` to `null` and stores one `default` `shared-iterations` scenario with `vus: 1` and `iterations: 1`

#### Scenario: Local execution can omit the archive without changing its run

- **GIVEN** `archive.js`
- **WHEN** the user runs `k6 cloud run --local-execution --no-archive-upload --once archive.js`
- **THEN** the `start-local-execution` request contains the scenario created by `--once` and `archive_size: null`
- **AND** k6 sends no request to a presigned archive upload URL
- **AND** local execution prints `API-RAN` once and reports one complete iteration

### Requirement: `--once` validates the configuration it runs

`--once` MUST ignore errors caused only by a discarded load combination. It MUST still fail when k6 cannot read a value, does not recognize the declared executor, or cannot find the selected function.

#### Scenario: Invalid discarded load values do not block the run

- **GIVEN** a `shared-iterations` scenario named `api` with `exec: 'api'`, `vus: 5`, and `iterations: 1`, which is invalid without `--once` because it has more VUs than iterations
- **AND** its `api` function prints `API-RAN`
- **WHEN** the user runs it with `k6 run --once`
- **THEN** k6 validates the replacement scenario, prints `API-RAN` once, and reports one complete iteration

#### Scenario: Invalid field values and unknown executors still fail

- **GIVEN** two scripts that export an `api` function which prints `API-RAN` and declare one scenario named `api` with `exec: 'api'`
- **AND** one scenario has `executor: 'shared-iterations'`, `vus: 'not-a-number'`, and `iterations: 1`, while the other has `executor: 'not-an-executor'`
- **WHEN** the user runs each script with `k6 run --once`
- **THEN** k6 rejects each invalid configuration and exits with a code other than zero
- **AND** `API-RAN` does not appear

#### Scenario: Missing required functions still fail the run

- **GIVEN** these fixtures:

  | Configuration | Exports | Required error text |
  | --- | --- | --- |
  | No scenarios | only `api`, which prints `API-RAN` | `executor default: function 'default' not found in exports` |
  | One scenario named `ui`, with no `exec` | only `api`, which prints `API-RAN` | `executor ui: function 'default' not found in exports` |
  | One scenario named `ui`, with `exec: 'target'` | only `api`, which prints `API-RAN` | `executor ui: function 'target' not found in exports` |

- **WHEN** the user runs each fixture with `k6 run --once`
- **THEN** each invocation reports its required text and exits with a code other than zero
- **AND** `API-RAN` does not appear

### Requirement: `--once` rejects multiple scenarios

With two or more effective scenarios, k6 MUST run init context to read the options, then reject `--once` before setup, VU initialization, browser launch, or any cloud request.

The error MUST state that `--once` can run only with one scenario.

#### Scenario: Exactly two scenarios fail through every run path

- **GIVEN** `multi.js` prints `INIT-RAN` in init context and declares scenarios named `zulu` and `xray`
- **AND** setup and both scenario functions print distinct markers
- **AND** `xray` is a browser scenario that calls the browser API
- **AND** the user builds `multi.tar` from `multi.js`
- **AND** `K6_BROWSER_EXECUTABLE_PATH=/does/not/exist/chromium`
- **WHEN** the user invokes each path:

  | Path | Input |
  | --- | --- |
  | `k6 run --once` | `multi.js` |
  | `k6 run --once` | `multi.tar` |
  | `k6 cloud run --once` | `multi.js` or `multi.tar` |
  | `k6 cloud run --local-execution --once` | `multi.js` or `multi.tar` |

- **THEN** each invocation reports that `--once` can run only with one scenario and exits with a code other than zero
- **AND** the log contains `INIT-RAN` but no setup or body markers
- **AND** the log does not mention `/does/not/exist/chromium` or a browser executable error
- **AND** no request reaches a cloud API

#### Scenario: `K6_ITERATIONS` does not hide multiple scenarios

- **GIVEN** `multi.js`
- **WHEN** the user runs `K6_ITERATIONS=3 k6 run --once multi.js`
- **THEN** k6 reports that `--once` can run only with one scenario
- **AND** neither scenario function marker appears

### Requirement: `--once` rejects CLI load flags

k6 MUST reject a command line that combines `--once` with `--vus`/`-u`, `--iterations`/`-i`, `--duration`/`-d`, or `--stage`/`-s`.

k6 MUST identify `--once` and the conflicting flag by its long name.

A value of zero MUST still count as a conflict.

#### Scenario: Every long and short load flag fails

- **GIVEN** `api.js`, which `--once` accepts on its own
- **WHEN** the user runs `k6 run --once <added flag> api.js` for each row:

  | Added flag | Conflicting long flag |
  | --- | --- |
  | `--vus 1` or `-u 2` | `--vus` |
  | `--iterations 2` or `-i 0` | `--iterations` |
  | `--duration 2s` or `-d 2s` | `--duration` |
  | `--stage 2s:2` or `-s 2s:2` | `--stage` |

- **THEN** every invocation exits with a code other than zero and `API-RAN` does not appear
- **AND** the error identifies `--once` and the conflicting long flag

#### Scenario: Cloud commands reject a CLI load flag with `--once`

- **GIVEN** `api.js`
- **WHEN** the user runs `k6 cloud run --once --iterations 2 api.js`
- **AND** separately runs `k6 cloud run --local-execution --once --iterations 2 api.js`
- **THEN** each command rejects the invocation and identifies the conflict between `--once` and `--iterations`
- **AND** `API-RAN` does not appear and no cloud request starts

### Requirement: Only bare CLI `--once` turns the behavior on

`--once` MUST be off by default. Only the bare form `--once` on the current command may turn the behavior on. It does not select a scenario.

An exported `once` option, `K6_ONCE`, `ONCE`, a JSON `once` field, or an archive option named `once` MUST NOT turn the behavior on.

#### Scenario: Without `--once`, k6 keeps the original load

- **GIVEN** `api.js`
- **WHEN** the user runs `k6 run api.js`
- **THEN** k6 lists `api` as `4 iterations shared among 4 VUs`
- **AND** the run reports `4 complete and 0 interrupted iterations`

#### Scenario: Without `--once`, `K6_ITERATIONS` replaces script scenarios

- **GIVEN** a script declares one `shared-iterations` scenario named `api` with `exec: 'api'`, four VUs, and four iterations
- **AND** its `api` function prints `API-RAN`
- **AND** its default function prints `FALLBACK-RAN`
- **WHEN** the user runs it without `--once` and with `K6_ITERATIONS=2`
- **THEN** the environment load replaces the `api` scenario
- **AND** `FALLBACK-RAN` appears twice and `API-RAN` does not appear

#### Scenario: Scripts, environment, configs, and archives cannot turn the behavior on

- **GIVEN** `api.js` declares four VUs and four iterations
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
- **AND** no run changes the load to one VU and one iteration

#### Scenario: `--once=api` cannot select a scenario

- **GIVEN** `script.js` is a valid test script whose default function prints `DEFAULT-RAN`
- **WHEN** the user runs `k6 run --once=api script.js`
- **THEN** k6 rejects the invocation as an invalid value for the `once` flag and exits with a code other than zero
- **AND** `DEFAULT-RAN` does not appear

### Requirement: Archive and cloud upload reject `--once`

`k6 archive` and `k6 cloud upload` MUST reject `--once` as an unknown flag.

#### Scenario: Archive and cloud upload reject the flag

- **GIVEN** `api.js`
- **WHEN** the user runs `k6 archive --once api.js`
- **AND** separately runs `k6 cloud upload --once api.js`
- **THEN** each command reports `unknown flag: --once`
- **AND** each command exits with a code other than zero
