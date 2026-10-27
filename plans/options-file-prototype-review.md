# Review: options file prototype plan

Review of [options-file-prototype.md](options-file-prototype.md) against `a435c42fe`.
Claims marked "verified" were checked with a small Go program or with a locally built k6.
The detailed sub-agent reports are in `/tmp/k6review/out_A.md` (per option), `out_B.md` (runtime and loading) and `out_D.md` (validation parity), which are not kept across reboots.
Section 6 was checked against the prototype in `554322ef7`, with k6 built from that commit.

## 1. Findings for the prototype

### 1.1 Options that should not be in the file

All four can be set in script options today as well, so the file does not introduce them.
The file makes them more visible, because it is plain data that platforms store, read and pass around.

- `tlsAuth`: `TLSAuth.UnmarshalJSON` parses the certificate while decoding (`lib/options.go:137`), so the file must contain the PEM certificate, the key and the password inline.
  Scripts load them with `open()` today, which a static file cannot do.
  The material is printed by `k6 inspect` and written into `metadata.json`, because `TLSAuth` has no custom marshaller.
  The plan lists references to files and secrets as a non-goal, so this is known, but it means `tlsAuth` is not usable in practice in the prototype.
- `cloud.token`: a secret in a per-test file, also printed by `inspect` and stored in archives.
  The rest of `cloud` is opaque data for the cloud backend, and k6 itself only reads `projectID`, `name`, `token` and `stackID` (`cloudapi/config.go:288`).
- `executionSegment`, `executionSegmentSequence`: they describe one instance of a distributed run, not the test.
  In a per-test file every instance would run the same segment, so they belong to the orchestrator.
  Env cannot override them (`ignored:"true"`, `lib/options.go:248`), only CLI flags can.
- `paused: true`: the test waits until it is resumed through the REST API, which stalls unattended platform runs.

### 1.2 Custom top-level keys

Unknown top-level keys in script options only produce a warning (`internal/js/bundle.go:220`), and some scripts keep their own data there, for example `options.myCustom`, and read it in VU code.
The plan makes unknown keys in the file an error, so such keys cannot move to the file, and `ext` is the place for them.
This is the only thing that script options accept today and the file rejects.

### 1.3 The exported `options` object in VU code is an existing problem

The plan says that VU code reading `options` sees the effective values.
That is already not true today, and the options file does not change the mechanism.

- `manipulateOptions` (`internal/js/bundle.go:284`) only writes keys that are set in the effective options and never deletes keys.
  Verified today: `k6 run --iterations 1` on a script with `duration: '1s'` leaves `options.duration` at `1s` in VU code, and `--once` does the same.
- Complex values are written as Go values.
  Verified with a plain `k6 run`: `Object.keys(options.scenarios)` returns Go methods such as `getSortedConfigs`, and `options.scenarios.<name>` is `undefined`.
- A frozen `options` export already fails today with "Cannot add property systemTags, object is not extensible" (verified).

What the mode changes is frequency.
Stale keys appear whenever the script sets a key that the file does not set, which is the normal case in the mode instead of an edge case.
For the prototype it is enough to correct the sentence in the plan and to name `exec.test.options` as the reliable view of the effective options.
Section 6.1 describes what the prototype shows for init code, `setup()` and `handleSummary()`.

## 2. Options that cannot be set in the file

None of these can be set in script options today either, so the file breaks nothing here.

- `consoleOutput`, `localIPs`: both fields of `lib.Options` carry the struct tag `json:"-"` (`lib/options.go:350`, `353`), which tells `encoding/json` to skip them.
  They can only be set with `--console-output` and `--local-ips` or with `K6_CONSOLE_OUTPUT` and `K6_LOCAL_IPS`.
  In script options they are dropped with the "unknown fields" warning, and in the `-c` file they are dropped silently.
  In the options file, strict decoding reports them as unknown fields (verified).
  Both describe the machine that runs the test, a local log file path and IP addresses of the host, so they do not belong in a per-test file anyway.
- `out`, `linger`, `noUsageReport`, `webDashboard`, `noArchiveUpload`, `collectors`: fields of `cmd.Config`, settable through `-c`, env and CLI, not through script options.
- Runtime options `env`, `compatibilityMode`, `includeSystemEnvVars`, `noThresholds`, `summaryMode`, `summaryExport`, `tracesOutput` and `type`: CLI and env only, because they must be known before init.
- `features`: decodes, but `Options.Apply` never merges it, and the plan ignores it with a warning as for script options.

## 3. Things that can only be validated at runtime (future)

There are things `k6 inspect --options-file` cannot check, so it cannot be sure that a test will run successfully.
They fall into two groups.
Only the first group really needs runtime evaluation.
The second group is static, but k6 checks it only on the `run` path or not at all, so it could later be moved into shared validation.
Both groups are out of scope for the prototype.

### 3.1 Needs the script, init or the running test

- `scenarios.*.exec`: whether the export exists and is callable is only known after init (`populateExports`).
  An accept-all `isExecutable` in `inspect` also hides a missing `export default` when the file uses shortcuts.
- Thresholds on custom metrics and on metrics that modules register at init, for example `browser_*` and `grpc_streams*`, because the registry knows them only after init.
- `scenarios.*.options.browser`: a free-form map whose `type` the browser module checks at the first browser iteration (`internal/js/modules/k6/browser/common/browser_options.go:57`), and it only works if the script imports `k6/browser`.
- Everything that happens at VU creation and in init: setup and teardown being functions, "no exported functions", TLS configuration of VUs.

### 3.2 Static, but checked only on the run path or nowhere

- The metric-name syntax of threshold keys, such as `http_req_duration{status:200`, is checked by `ParseMetricName` inside `Thresholds.Validate` (`metrics/thresholds.go:245`), which needs a registry.
- Thresholds on built-in metrics: existence and aggregation method are certain without init, because built-in metrics are registered before init with fixed types (`metrics/builtin.go:77`, `metrics/registry.go:57`).
  Example: `checks: ["p(95)<1"]` parses, but `run` rejects it.
- DNS TTL is validated only in `Runner.SetOptions` (`internal/js/runner.go:607`).
- Decoding of `options.cloud`, required system tags and push concurrency are checked only when the cloud output is created (`internal/output/cloud/output.go:143`).
- Not validated anywhere when set from JSON: `summaryTimeUnit` (CLI only), a negative `metricSamplesBufferSize` (panics at `internal/cmd/run.go:293`), stages without `duration` (silently dropped) and empty tag names.

### 3.3 Implementation notes for step 1

These two matter for the prototype even with parity deferred.

- `Thresholds.Validate(name, nil)` panics (`metrics/registry.go:112`), so "nil registry means parse only" must skip the `Validate` call entirely.
- The body of `consolidateDeriveAndValidateConfig` also reads `RuntimeOptions.NoThresholds` (`internal/cmd/test_load.go:607`) and the feature flags (`test_load.go:582`, `631`, `internal/cmd/config.go:249`).
  The shared function needs both as inputs, and `inspect` in the mode has to call `getRuntimeOptions` and resolve the feature flags itself.
  A nil `*features.Flags` panics in `Tags()` (`internal/features/features.go:61`).

## 4. Deferred, keep in mind for later

### 4.1 Decoding is not strict enough for a platform contract

Go's `encoding/json` does not support the claim that platforms can validate the file in their backend with the same result as k6 (P5).
All points below were verified.

- Keys match case-insensitively and duplicate keys resolve to the last value.
  `{"vus":1,"VUS":50}` gives 50 VUs, while a case-sensitive JS or TS validator such as SM's sees 1, which is a policy bypass.
  This also works inside scenario configs.
- Data after the first JSON value is ignored, so `{"vus":1} garbage` decodes.
- Unknown fields are rejected only at the top level and in types that use `lib.StrictJSONUnmarshal` (scenarios, TLS types).
  Typos in nested keys are dropped silently: stages (`traget`), `dns` (`selct`) and threshold objects (`abortOnFial`).
  `systemTags` drops unknown names, and an empty set means no system tags at all.
  `hosts` turns invalid IPs into nil.
  `cloud`, `ext` and `scenarios.*.options.browser` are opaque.
- A possible fix is a canonical decoder that requires exact case, rejects duplicate keys and trailing data, and is strict in nested types, or a published schema together with the Go validator.

### 4.2 Automatic extension resolution

- The provisioned binary is started with the same arguments (`internal/cmd/launcher.go:65`), and its k6 version is chosen by the build service, so it may not know `--options-file`.
- The file is read twice, by the parent and by the child.
  With process substitution (`<(...)`) or `/dev/stdin`, the child reads an empty file.
- Warnings from the parent would be printed twice.
- Possible fixes: reject files that are not regular files, and add a minimum k6 version to the dependencies in the mode.

### 4.3 k6-operator needs an inspect branch that does not run init

Today the operator's initializer runs `k6 archive <script> <spec.arguments> -O a.tar` and then `k6 inspect --execution-requirements a.tar` (`pkg/resources/jobs/initializer.go` in grafana/k6-operator).
User CLI flags such as `--vus` therefore end up in the archive's options and in `maxVUs` and `totalDuration`, and init runs twice.
`k6 inspect --options-file` has no CLI option flags, so using it instead would drop `spec.arguments`.
For the mode to help the operator, `inspect` needs a branch for archives that does not run init: it reads `arc.Options` from `metadata.json`, which is already authoritative, and consolidates and derives from there.
This is the quirk noted at the end of the plan.
The flow `k6 archive --options-file opts.json <args>` followed by `k6 inspect --execution-requirements a.tar` would then run init once instead of twice and keep the arguments.
Running no init at all would additionally need CLI option flags on `inspect`, which is problem 1.

## 5. More findings

- Comparison warning, false differences: `cloud` and `ext` are raw JSON that keeps the file's key order, while the script side arrives with sorted keys.
  Compare after decoding both sides into `any` with `reflect.DeepEqual`.
- Comparison warning, noise: platforms that always pass the flag, and scripts with `__ENV`-dependent options, would get it on every run.
- Traps when capturing the script options in `populateExports`:
  - an early `return` inside the export loop drops the remaining entries from `callableExports`,
  - decoding into `b.Options` merges the script's keys into the file's options,
  - `export let options;` decodes as empty options,
  - the "unknown fields in the options exported in the script" warning would still fire.
- `inspect` in the mode never reads its argument, so `inspect --options-file o.json test.tar`, or a path that does not exist, succeeds while `run` with the same arguments fails.
  Reject the argument in the mode or warn that it is not read.
  Still the case in the prototype: the mode only relaxes `Args` to `cobra.RangeArgs(0, 1)` in `internal/cmd/inspect.go`, and `inspectOptionsFile` never looks at `args`.
- Strict decoding and version skew: SM runs several k6 versions, so a key added in a newer version is a hard error on an older runner, and the validator's version can differ from the runner's.
- Layer order for platforms: `-c` sits below the file, so platform policy such as `blacklistIPs` or `insecureSkipTLSVerify` cannot live in `-c`.
  `K6_VUS` and `--vus` above the file still replace the file's scenarios (#3741).
- JSON is not quite the same shape as the JS export: `10.0` and `1e1` are rejected for integer fields while JS accepts them, a byte-order mark fails parsing, and an empty file gives a bare `EOF` error (verified).
- Several messages hard-code "script": `validateConfig`, `selectScenarios`, `beautifyOptionsJSONUnmarshalError` and `warnOnScriptOptionsFeatures`.
- pflag silently keeps the last value of a repeated flag, so "passing it more than once is an error" needs a custom `pflag.Value`.
- Resolve a relative path with `gs.Getwd()`, as `readSource` does, because the tests use an in-memory filesystem with cwd `/test/`.
- Read the file through `gs.FS` and not through the caching filesystem from `loader.CreateFilesystems`, otherwise it ends up inside the archive.
- Existing env bugs found on the way (verified): an unprefixed `CLOUD` or `COLLECTORS` environment variable overrides `options.cloud` or the collectors config, and `K6_HOSTS` is never read, because envconfig looks up `K6_HOSTS_VALID`.

## 6. Findings from the prototype

### 6.1 Script code sees options that never took effect

- Init code sees only the script's own literal.
  `exec.test.options` throws "getting test options in the init context is not supported" there (`internal/js/modules/k6/execution/execution.go:163`).
  In the mode, init code therefore has no view of the effective options at all, for example a `SharedArray` sized by `options.vus` uses the script's value.
- VU code, `setup()` and `handleSummary()` see a mix of both sources.
  `manipulateOptions` (`internal/js/bundle.go:309`) writes the set keys of the effective options into the script's object and keeps all other keys.
  `handleSummary()` runs in a fresh VU (`internal/js/runner.go:397`), so it gets the same mix.
- Verified: with a script that exports `{vus: 10, duration: '1m', thresholds: {http_req_failed: [...]}}` and a file with `{"iterations": 3, "thresholds": {"iterations": [...]}}`, `handleSummary()` sees `vus: 10` and `duration: '1m'` from the script next to `iterations: 3` and the file's thresholds.
  The test ran 3 iterations with 1 VU.
- Unlike in 1.3, the stale values do not come from a lower layer that a higher one overrode, but from a source that is not part of the configuration at all.
  Report code that serializes `options`, for example a custom `handleSummary()` report, is therefore wrong in the mode.
- `exec.test.options` gives the effective options in VU code, `setup()` and `handleSummary()` (verified), but not in init.
- Possible directions:
  - The file is known before init, so the mode could consolidate before init and make `exec.test.options` available in init.
  - The mode could leave the exported object untouched, so that it at least shows only the script's own values and not a mix.

### 6.2 Option to think about: the export is still evaluated

- In the mode "ignored" means "not used", not "not evaluated".
  Init still runs the script's top-level code, including the expression that builds the `options` export.
- `open()`, `secrets.get()` and imports that only the export uses still run, and fail if the file, the secret or the module is missing, although the result is discarded.
- JS evaluation cannot skip the initializer of a single export, so k6 cannot avoid this while it runs the script.
- Options to think about:
  - Keep the warning and document that the export is evaluated but not used (the prototype's behaviour).
  - Reject scripts that export `options` in the mode.
    This makes the file the only source and avoids the evaluation, but forces users to delete the export, and the script then runs with defaults without the flag.

### 6.3 Nothing records the options file or the options from it

- The banner (`printExecutionDescription`, `internal/cmd/ui.go:117`) prints `script: test.js` and the derived scenarios, but not the options file.
- The machine-readable summary (`--new-machine-readable-summary`) has a `config.script` field, which k6 fills with the script source (`internal/cmd/run.go:236`).
  In the mode it shows the ignored export, and nothing records the file or the options that were used.
  Verified: the summary of a run with 3 iterations and 1 VU contains `vus: 10, duration: '1m'` from the script.
- The summary data passed to `handleSummary()` and written by `--summary-export` has only `summaryTrendStats`, `summaryTimeUnit` and `noColor` in `data.options`.
- In script mode the embedded script at least contains the test's options layer.
  In the mode, a CI artifact or a platform that stores summaries cannot tell from the summary what ran.
- Recording the options needs:
  - a new schema version in grafana/k6-summary, because `config` has `additionalProperties: false` in `schemas/summary/1.0.0/schema.json`,
  - a choice between consolidated options, which can be fed back as an options file, and derived options, which describe what ran but are rejected as an options file because they contain both `stages` and `scenarios`,
  - redaction of `tlsAuth` material and `cloud.token`.
- An `options: opts.json` line in the banner is cheap and does not depend on the schema.

### 6.4 Risk: cloud test identity if the mode is extended to `k6 cloud`

The prototype does not add the flag to `k6 cloud run` and `k6 cloud upload`.
With the flag added to both in a throwaway copy (two lines), the following was verified against the v6 mock server in `internal/cloudapi/v6/v6test`.

- The cloud test's name and project come from `options.cloud.name` and `options.cloud.projectID`, which in the mode come only from the file.
  If the file does not repeat them, the name falls back to the script's file name (`internal/cmd/cloud.go:256`), and the project to the stack's default project with only a warning.
  Verified: a script that exports `cloud.name: 'from-script'` and a file without `cloud.name` are uploaded as `test.js`.
- Moving an existing test to the mode can therefore create a new cloud test, possibly in another project, and split its history.
- The reverse also happens: several per-environment files for one script map to the same cloud test.
  On a name conflict, `UploadTest` falls back to `updateScript` (`internal/cloudapi/v6/api.go:278`), so each upload replaces the stored archive, and runs with different load profiles share one history.
- Possible mitigations: warn when the script's export sets `cloud` keys that the file lacks, or derive the default name from both the script and the file.
