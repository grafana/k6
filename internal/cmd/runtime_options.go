package cmd

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/lib/summary"
	"go.k6.io/k6/lib"
)

// TODO: move this whole file out of the cmd package? maybe when fixing
// https://github.com/k6io/k6/issues/883, since this code is fairly
// self-contained and easily testable now, without any global dependencies...

var userEnvVarName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func runtimeOptionFlagSet(includeSysEnv bool) *pflag.FlagSet {
	flags := pflag.NewFlagSet("", 0)
	flags.SortFlags = false
	flags.Bool("include-system-env-vars", includeSysEnv, "pass the real system environment variables to the runtime")
	flags.String("compatibility-mode", "extended",
		`JavaScript compiler compatibility mode, "extended" or "base"
base: pure Sobek - Golang JS VM supporting ES6+
extended: base + sets "global" as alias for "globalThis"
`)
	flags.StringP("type", "t", "", "override test type, \"js\" or \"archive\"")
	flags.StringArrayP("env", "e", nil, "add/override environment variable with `VAR=value`")
	flags.Bool("no-thresholds", false, "don't run thresholds")
	// TODO(@joanlopez): remove by k6 v2.0, once we completely drop the support of the deprecated --no-summary flag.
	flags.Bool("no-summary", false, "don't show the summary at the end of the test")
	if err := flags.MarkDeprecated("no-summary", "use --summary-mode=disabled instead"); err != nil {
		panic(err) // Should never happen
	}
	flags.String("summary-mode", summary.ModeCompact.String(), "determine the summary mode,"+
		" \"compact\", \"full\" or \"disabled\"")
	flags.String(
		"summary-export",
		"",
		"output the end-of-test summary report to JSON file",
	)
	// TODO(@joanlopez): remove by k6 v2.0, once the new summary model is the default and the only one.
	flags.Bool("new-machine-readable-summary", false, "enables the new machine-readable summary, "+
		"which is used for summary exports and as handleSummary() argument")
	flags.String("traces-output", "none",
		"set the output for k6 traces, possible values are none,otel[=host:port]")
	flags.Bool("js-profiling-enabled", false, "enable JS execution observability captures (experimental)")
	flags.String("js-profiling-scope", "", "set JS profiling scope: init|vu|combined (default: combined)")
	flags.String("js-cpu-profile-output", "", "write JS-attributed CPU profile to file path")
	flags.String("js-runtime-trace-output", "", "write JS-attributed runtime trace to file path")
	flags.String("js-profile-id", "", "set a custom profile correlation id")
	flags.String("js-first-runner-mem-max-bytes", "", "track first-runner JS memory milestones against max bytes (supports suffixes like kb, mb, gb)")
	flags.Int64("js-first-runner-mem-step-percent", 5, "first-runner memory milestone step percentage of max bytes")
	return flags
}

func getRuntimeOptions(
	flags *pflag.FlagSet,
	environment map[string]string,
) (lib.RuntimeOptions, error) {
	// TODO: refactor with composable helpers as a part of #883, to reduce copy-paste
	// TODO: get these options out of the JSON config file as well?
	opts, err := populateRuntimeOptionsFromEnv(runtimeOptionsFromFlags(flags), environment)
	if err != nil {
		return opts, err
	}

	// Set/overwrite environment variables with custom user-supplied values
	envVars, err := flags.GetStringArray("env")
	if err != nil {
		return opts, err
	}
	for _, kv := range envVars {
		k, v := state.ParseEnvKeyValue(kv)
		// Allow only alphanumeric ASCII variable names for now
		if !userEnvVarName.MatchString(k) {
			return opts, fmt.Errorf("invalid environment variable name '%s'", k)
		}
		opts.Env[k] = v
	}
	if opts.JSFirstRunnerMemMaxBytes.Valid {
		if _, err = parseByteSize(opts.JSFirstRunnerMemMaxBytes.String); err != nil {
			return opts, fmt.Errorf("invalid js-first-runner-mem-max-bytes value: %w", err)
		}
	}

	return opts, nil
}

func runtimeOptionsFromFlags(flags *pflag.FlagSet) lib.RuntimeOptions {
	opts := lib.RuntimeOptions{
		TestType:                  getNullString(flags, "type"),
		IncludeSystemEnvVars:      getNullBool(flags, "include-system-env-vars"),
		CompatibilityMode:         getNullString(flags, "compatibility-mode"),
		NoThresholds:              getNullBool(flags, "no-thresholds"),
		NoSummary:                 getNullBool(flags, "no-summary"),
		SummaryMode:               getNullString(flags, "summary-mode"),
		SummaryExport:             getNullString(flags, "summary-export"),
		NewMachineReadableSummary: getNullBool(flags, "new-machine-readable-summary"),
		TracesOutput:              getNullString(flags, "traces-output"),
		JSProfilingEnabled:        getNullBool(flags, "js-profiling-enabled"),
		JSProfilingScope:          getNullString(flags, "js-profiling-scope"),
		JSCPUProfileOutput:        getNullString(flags, "js-cpu-profile-output"),
		JSRuntimeTraceOutput:      getNullString(flags, "js-runtime-trace-output"),
		JSProfileID:               getNullString(flags, "js-profile-id"),
		JSFirstRunnerMemMaxBytes:  getNullString(flags, "js-first-runner-mem-max-bytes"),
		JSFirstRunnerMemStepPercent: getNullInt64(
			flags,
			"js-first-runner-mem-step-percent",
		),
		Env: make(map[string]string),
	}
	return opts
}

func populateRuntimeOptionsFromEnv(opts lib.RuntimeOptions, environment map[string]string) (lib.RuntimeOptions, error) {
	// Only override if not explicitly set via the CLI flag

	if envVar, ok := environment["K6_TYPE"]; !opts.TestType.Valid && ok {
		opts.TestType = null.StringFrom(envVar)
	}

	if envVar, ok := environment["K6_COMPATIBILITY_MODE"]; !opts.CompatibilityMode.Valid && ok {
		opts.CompatibilityMode = null.StringFrom(envVar)
	}

	if _, err := lib.ValidateCompatibilityMode(opts.CompatibilityMode.String); err != nil {
		// some early validation
		return opts, err
	}

	if err := saveBoolFromEnv(environment, "K6_INCLUDE_SYSTEM_ENV_VARS", &opts.IncludeSystemEnvVars); err != nil {
		return opts, err
	}

	if err := saveBoolFromEnv(environment, "K6_NO_THRESHOLDS", &opts.NoThresholds); err != nil {
		return opts, err
	}

	if err := saveBoolFromEnv(environment, "K6_NO_SUMMARY", &opts.NoSummary); err != nil {
		return opts, err
	}

	if envVar, ok := environment["K6_SUMMARY_MODE"]; !opts.SummaryMode.Valid && ok {
		opts.SummaryMode = null.StringFrom(envVar)
	}

	if _, err := summary.ValidateMode(opts.SummaryMode.String); err != nil {
		// In the case of an invalid summary mode, we early stop
		// the execution and return the error to the user.
		return opts, err
	}

	if envVar, ok := environment["K6_SUMMARY_EXPORT"]; !opts.SummaryExport.Valid && ok {
		opts.SummaryExport = null.StringFrom(envVar)
	}

	if err := saveBoolFromEnv(
		environment, "K6_NEW_MACHINE_READABLE_SUMMARY", &opts.NewMachineReadableSummary,
	); err != nil {
		return opts, err
	}

	if envVar, ok := environment["SSLKEYLOGFILE"]; !opts.KeyWriter.Valid && ok {
		opts.KeyWriter = null.StringFrom(envVar)
	}

	if envVar, ok := environment["K6_TRACES_OUTPUT"]; !opts.TracesOutput.Valid && ok {
		opts.TracesOutput = null.StringFrom(envVar)
	}
	if err := saveBoolFromEnv(environment, "K6_JS_PROFILING_ENABLED", &opts.JSProfilingEnabled); err != nil {
		return opts, err
	}
	if envVar, ok := environment["K6_JS_PROFILING_SCOPE"]; !opts.JSProfilingScope.Valid && ok {
		opts.JSProfilingScope = null.StringFrom(envVar)
	}
	if envVar, ok := environment["K6_JS_CPU_PROFILE_OUTPUT"]; !opts.JSCPUProfileOutput.Valid && ok {
		opts.JSCPUProfileOutput = null.StringFrom(envVar)
	}
	if envVar, ok := environment["K6_JS_RUNTIME_TRACE_OUTPUT"]; !opts.JSRuntimeTraceOutput.Valid && ok {
		opts.JSRuntimeTraceOutput = null.StringFrom(envVar)
	}
	if envVar, ok := environment["K6_JS_PROFILE_ID"]; !opts.JSProfileID.Valid && ok {
		opts.JSProfileID = null.StringFrom(envVar)
	}
	if envVar, ok := environment["K6_JS_FIRST_RUNNER_MEM_MAX_BYTES"]; !opts.JSFirstRunnerMemMaxBytes.Valid && ok {
		opts.JSFirstRunnerMemMaxBytes = null.StringFrom(envVar)
	}
	if envVar, ok := environment["K6_JS_FIRST_RUNNER_MEM_STEP_PERCENT"]; !opts.JSFirstRunnerMemStepPercent.Valid && ok {
		v, err := strconv.ParseInt(envVar, 10, 64)
		if err != nil {
			return opts, fmt.Errorf("env var 'K6_JS_FIRST_RUNNER_MEM_STEP_PERCENT' is not a valid integer value: %w", err)
		}
		opts.JSFirstRunnerMemStepPercent = null.IntFrom(v)
	}

	// If enabled, gather the actual system environment variables
	if opts.IncludeSystemEnvVars.Bool {
		opts.Env = environment
	}

	return opts, nil
}

func parseByteSize(v string) (int64, error) {
	s := strings.TrimSpace(strings.ToLower(v))
	if s == "" {
		return 0, fmt.Errorf("empty value")
	}

	type unitDef struct {
		suffix string
		mult   int64
	}
	units := []unitDef{
		{suffix: "tib", mult: 1 << 40},
		{suffix: "gib", mult: 1 << 30},
		{suffix: "mib", mult: 1 << 20},
		{suffix: "kib", mult: 1 << 10},
		{suffix: "tb", mult: 1000 * 1000 * 1000 * 1000},
		{suffix: "gb", mult: 1000 * 1000 * 1000},
		{suffix: "mb", mult: 1000 * 1000},
		{suffix: "kb", mult: 1000},
		{suffix: "b", mult: 1},
	}
	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			n := strings.TrimSpace(strings.TrimSuffix(s, u.suffix))
			if n == "" {
				return 0, fmt.Errorf("missing number in %q", v)
			}
			base, err := strconv.ParseInt(n, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid number in %q: %w", v, err)
			}
			if base < 0 {
				return 0, fmt.Errorf("value must be >= 0")
			}
			return base * u.mult, nil
		}
	}

	base, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unsupported size format %q", v)
	}
	if base < 0 {
		return 0, fmt.Errorf("value must be >= 0")
	}
	return base, nil
}

func saveBoolFromEnv(env map[string]string, varName string, placeholder *null.Bool) error {
	strValue, ok := env[varName]
	if !ok {
		return nil
	}
	val, err := strconv.ParseBool(strValue)
	if err != nil {
		return fmt.Errorf("env var '%s' is not a valid boolean value: %w", varName, err)
	}
	// Only override if not explicitly set via the CLI flag
	if !placeholder.Valid {
		*placeholder = null.BoolFrom(val)
	}
	return nil
}
