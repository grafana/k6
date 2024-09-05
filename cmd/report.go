package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"runtime"

	"go.k6.io/k6/execution"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/usage"
)

func createReport(u *usage.Usage, execScheduler *execution.Scheduler) map[string]any {
	execState := execScheduler.GetState()
	m := u.Map()

	m["k6_version"] = consts.Version
	m["duration"] = execState.GetCurrentTestRunDuration().String()
	m["goos"] = runtime.GOOS
	m["goarch"] = runtime.GOARCH
	m["vus_max"] = uint64(execState.GetInitializedVUsCount())
	m["iterations"] = execState.GetFullIterationCount()
	executors := make(map[string]int)
	for _, ec := range execScheduler.GetExecutorConfigs() {
		executors[ec.GetType()]++
	}
	m["executors"] = executors

	return m
}

func reportUsage(ctx context.Context, execScheduler *execution.Scheduler, test *loadedAndConfiguredTest) error {
	m := createReport(test.preInitState.Usage, execScheduler)
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://reports.k6.io", bytes.NewBuffer(body))
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
