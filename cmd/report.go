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

func createReport(u *usage.Usage, execScheduler *execution.Scheduler) (map[string]any, error) {
	executors := make(map[string]int)
	for _, ec := range execScheduler.GetExecutorConfigs() {
		executors[ec.GetType()]++
	}
	execState := execScheduler.GetState()
	m, err := u.Map()

	if m != nil {
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
	}

	return m, err
}

func reportUsage(ctx context.Context, execScheduler *execution.Scheduler, test *loadedAndConfiguredTest) error {
	m, err := createReport(test.preInitState.Usage, execScheduler)
	if err != nil {
		// TODO actually log the error but continue if there is something to report
		return err
	}
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
