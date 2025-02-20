package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"

	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/internal/execution"
	"go.k6.io/k6/internal/usage"
)

func createReport(u *usage.Usage, execScheduler *execution.Scheduler) map[string]any {
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

	return m
}

func reportUsage(ctx context.Context, execScheduler *execution.Scheduler, test *loadedAndConfiguredTest) error {
	m := createReport(test.preInitState.Usage, execScheduler)
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}

	const usageStatsURL = "https://stats.grafana.org/k6-usage-report"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, usageStatsURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
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
