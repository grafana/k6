package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"strings"

	"go.k6.io/k6/execution"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/usage"
)

func createReport(
	u *usage.Usage, execScheduler *execution.Scheduler, importedModules []string, outputs []string,
) (map[string]any, error) {
	for _, ec := range execScheduler.GetExecutorConfigs() {
		err := u.Uint64("executors/"+ec.GetType(), 1)
		if err != nil { // TODO report it as an error but still send the report?
			return nil, err
		}
	}

	// collect the report only with k6 public modules
	for _, module := range importedModules {
		// Exclude JS modules extensions to prevent to leak
		// any user's custom extensions
		if strings.HasPrefix(module, "k6/x") {
			continue
		}
		// Exclude any import not starting with the k6 prefix
		// that identifies a k6 built-in stable or experimental module.
		// For example, it doesn't include any modules imported from the file system.
		if !strings.HasPrefix(module, "k6") {
			continue
		}
		_ = u.Strings("modules", module) // TODO this will be moved and the error can be logged there
	}

	builtinOutputs := builtinOutputStrings()

	// TODO: migrate to slices.Contains as soon as the k6 support
	// for Go1.20 will be over.
	builtinOutputsIndex := make(map[string]bool, len(builtinOutputs))
	for _, bo := range builtinOutputs {
		builtinOutputsIndex[bo] = true
	}

	// collect only the used outputs that are builtin
	for _, o := range outputs {
		// TODO:
		// if !slices.Contains(builtinOutputs, o) {
		// 	continue
		// }
		if !builtinOutputsIndex[o] {
			continue
		}
		_ = u.Strings("outputs", o) // TODO this will be moved and the error can be logged there
	}
	execState := execScheduler.GetState()
	m, err := u.Map()

	if m != nil {
		m["k6_version"] = consts.Version
		m["duration"] = execState.GetCurrentTestRunDuration().String()
		m["goos"] = runtime.GOOS
		m["goarch"] = runtime.GOARCH
		m["goarch"] = runtime.GOARCH
		m["goarch"] = runtime.GOARCH
		m["vus_max"] = uint64(execState.GetInitializedVUsCount())
		m["iterations"] = execState.GetFullIterationCount()
	}

	return m, err
}

func reportUsage(ctx context.Context, execScheduler *execution.Scheduler, test *loadedAndConfiguredTest) error {
	outputs := make([]string, 0, len(test.derivedConfig.Out))
	for _, o := range test.derivedConfig.Out {
		outputName, _ := parseOutputArgument(o)
		outputs = append(outputs, outputName)
	}

	m, err := createReport(test.usage, execScheduler, test.moduleResolver.Imported(), outputs)
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
