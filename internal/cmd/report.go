package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"

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

func createReport(u *usage.Usage, execScheduler *execution.Scheduler, catalog map[string]struct{}) map[string]any {
	execState := execScheduler.GetState()
	m := u.Map()

	m["k6_version"] = build.Version
	m["duration"] = execState.GetCurrentTestRunDuration().String()
	m["goos"] = runtime.GOOS
	m["goarch"] = runtime.GOARCH
	m["vus_max"] = uint64(execState.GetInitializedVUsCount()) //nolint:gosec
	m["iterations"] = execState.GetFullIterationCount()
	executors := make(map[string]int)
	for _, ec := range execScheduler.GetExecutorConfigs() {
		executors[ec.GetType()]++
	}
	m["executors"] = executors
	m["is_ci"] = isCI(execState.Test.LookupEnv)

	resolveExtensions(m, catalog)

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

// defaultUsageReportURL is the production endpoint the anonymous usage report
// is sent to when K6_USAGE_REPORT_URL is not set.
const defaultUsageReportURL = "https://stats.grafana.org/k6-usage-report"

func reportUsage(ctx context.Context, gs *state.GlobalState, execScheduler *execution.Scheduler, test *loadedAndConfiguredTest) error {
	m := createReport(test.preInitState.Usage, execScheduler, catalogModulePaths(ctx, gs))
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}

	usageStatsURL := defaultUsageReportURL
	if url, ok := test.preInitState.LookupEnv(state.UsageReportURL); ok && url != "" {
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
