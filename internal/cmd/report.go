package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/ext"
	"go.k6.io/k6/v2/internal/build"
	"go.k6.io/k6/v2/internal/execution"
	"go.k6.io/k6/v2/internal/usage"
)

// extensionEntry identifies a used extension in the usage report by its Go
// module path, along with its version and kind.
type extensionEntry struct {
	Module  string `json:"module"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

// newExtensionEntry builds the usage-report entry for a registered extension.
func newExtensionEntry(e *ext.Extension) extensionEntry {
	return extensionEntry{
		Module:  e.Path,
		Version: e.Version,
		Kind:    e.Type.String(),
	}
}

// envLookup adapts an environment map to the lookup-function shape the report
// helpers share with the run path's LookupEnv.
func envLookup(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		val, ok := env[key]
		return val, ok
	}
}

// addEnvironmentInfo stamps the fields identifying this k6 binary and where it
// runs. Both the run report and the subcommand report carry them.
func addEnvironmentInfo(m map[string]any, lookupEnv func(string) (string, bool)) {
	m["k6_version"] = build.Version
	m["goos"] = runtime.GOOS
	m["goarch"] = runtime.GOARCH
	m["is_ci"] = isCI(lookupEnv)
}

// createReport assembles the anonymous usage report for a run from its
// scheduler and recorded usage, keeping only the used extensions advertised in
// the public catalog. It is the run report k6 has always sent, with the
// extensions field added.
func createReport(u *usage.Usage, execScheduler *execution.Scheduler, catalog map[string]struct{}) map[string]any {
	execState := execScheduler.GetState()
	m := u.Map()

	addEnvironmentInfo(m, execState.Test.LookupEnv)

	m["duration"] = execState.GetCurrentTestRunDuration().String()
	m["vus_max"] = uint64(execState.GetInitializedVUsCount()) //nolint:gosec
	m["iterations"] = execState.GetFullIterationCount()
	executors := make(map[string]int)
	for _, ec := range execScheduler.GetExecutorConfigs() {
		executors[ec.GetType()]++
	}
	m["executors"] = executors

	resolveExtensions(m, catalog)

	return m
}

// createSubcommandReport builds the usage report for an invoked subcommand
// extension. A subcommand is not a load test, so it reports the identity fields
// and the used extension without the run summary that createReport derives from
// a scheduler.
func createSubcommandReport(ctx context.Context, gs *state.GlobalState, extension *ext.Extension) map[string]any {
	m := make(map[string]any)
	addEnvironmentInfo(m, envLookup(gs.Env))
	m["extensions"] = []any{extension}
	resolveExtensions(m, catalogModulePaths(ctx, gs))

	return m
}

// resolveExtensions replaces the used extensions recorded under "extensions" in
// usage with report entries, keeping only those advertised in the public
// catalog. Fail closed: dropping the raw list first means an empty catalog
// reports no extensions rather than leaking it, and the key stays omitted when
// nothing survives instead of sending [].
func resolveExtensions(m map[string]any, catalog map[string]struct{}) {
	used, ok := m["extensions"].([]any)
	if !ok {
		return
	}
	delete(m, "extensions")
	if entries := filterExtensions(used, catalog); len(entries) > 0 {
		m["extensions"] = entries
	}
}

// filterExtensions turns the recorded used extensions into report entries,
// keeping only those whose module path is advertised in the public catalog and
// de-duplicating per (module, kind).
func filterExtensions(used []any, public map[string]struct{}) []extensionEntry {
	seen := make(map[[2]string]struct{}, len(used))
	entries := make([]extensionEntry, 0, len(used))
	for _, v := range used {
		e, ok := v.(*ext.Extension)
		if !ok {
			continue
		}
		if _, ok := public[e.Path]; !ok {
			continue
		}
		key := [2]string{e.Path, e.Type.String()}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		entries = append(entries, newExtensionEntry(e))
	}
	return entries
}

// reportUsage sends the report built by create, logging the attempt and outcome
// at debug, all bounded by a timeout. Both k6 run and k6 x call it; whether to
// run it in the background is the caller's concern, not the pipeline's.
func reportUsage(ctx context.Context, gs *state.GlobalState, create func(ctx context.Context) map[string]any) {
	reportCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	gs.Logger.Debug("Sending usage report...")
	if err := postUsageReport(reportCtx, envLookup(gs.Env), create(reportCtx)); err != nil {
		gs.Logger.WithError(err).Debug("Error sending usage report")
	} else {
		gs.Logger.Debug("Usage report sent successfully")
	}
}

// defaultUsageReportURL is the production endpoint the anonymous usage report
// is sent to when K6_USAGE_REPORT_URL is not set.
const defaultUsageReportURL = "https://stats.grafana.org/k6-usage-report"

func postUsageReport(ctx context.Context, lookupEnv func(string) (string, bool), m map[string]any) error {
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}

	usageStatsURL := defaultUsageReportURL
	if url, ok := lookupEnv(state.UsageReportURL); ok && url != "" {
		usageStatsURL = url
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, usageStatsURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req) //nolint:gosec
	if err == nil {
		_ = res.Body.Close()
	}

	return err
}

// isCI is a helper that follows a naive approach to determine if k6 is being
// executed within a CI system. This naive approach consists of checking if
// the "CI" environment variable (among others) is set.
//
// We treat the "CI" environment variable carefully, because it's the one
// used more often, and because we know sometimes it's explicitly set to
// "false" to signal that k6 is not running in a CI environment.
//
// It is not a foolproof method, but it should work for most cases.
func isCI(lookupEnv func(key string) (val string, ok bool)) bool {
	if ci, ok := lookupEnv("CI"); ok {
		ciBool, err := strconv.ParseBool(ci)
		if err == nil {
			return ciBool
		}
		// If we can't parse the "CI" value as a bool, we assume return true
		// because we know that at least it's not set to any variant of "false",
		// which is the most common use case, and the reasoning we apply below.
		return true
	}

	// List of common environment variables used by different CI systems.
	ciEnvVars := []string{
		"BUILD_ID",               // Jenkins, Cloudbees
		"BUILD_NUMBER",           // Jenkins, TeamCity
		"CI",                     // Travis CI, CircleCI, Cirrus CI, Gitlab CI, Appveyor, CodeShip, dsari
		"CI_APP_ID",              // Appflow
		"CI_BUILD_ID",            // Appflow
		"CI_BUILD_NUMBER",        // Appflow
		"CI_NAME",                // Codeship and others
		"CONTINUOUS_INTEGRATION", // Travis CI, Cirrus CI
		"RUN_ID",                 // TaskCluster, dsari
	}

	// Check if any of these variables are set
	for _, key := range ciEnvVars {
		if _, ok := lookupEnv(key); ok {
			return true
		}
	}

	return false
}
