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
) map[string]any {
	for _, ec := range execScheduler.GetExecutorConfigs() {
		u.Count("executors/"+ec.GetType(), 1)
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
		u.Strings("modules", module)
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
		u.Strings("outputs", o)
	}

	u.String("k6_version", consts.Version)
	execState := execScheduler.GetState()
	u.Count("vus_max", execState.GetInitializedVUsCount())
	u.Count("iterations", int64(execState.GetFullIterationCount()))
	u.String("duration", execState.GetCurrentTestRunDuration().String())
	u.String("goos", runtime.GOOS)
	u.String("goarch", runtime.GOARCH)
	return u.Map()
}

func reportUsage(ctx context.Context, execScheduler *execution.Scheduler, test *loadedAndConfiguredTest) error {
	outputs := make([]string, 0, len(test.derivedConfig.Out))
	for _, o := range test.derivedConfig.Out {
		outputName, _ := parseOutputArgument(o)
		outputs = append(outputs, outputName)
	}

	body, err := json.Marshal(createReport(test.usage, execScheduler, test.moduleResolver.Imported(), outputs))
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
