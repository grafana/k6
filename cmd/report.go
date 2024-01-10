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
)

type report struct {
	Version    string         `json:"k6_version"`
	Executors  map[string]int `json:"executors"`
	VUsMax     int64          `json:"vus_max"`
	Iterations uint64         `json:"iterations"`
	Duration   string         `json:"duration"`
	GoOS       string         `json:"goos"`
	GoArch     string         `json:"goarch"`
	Modules    []string       `json:"modules"`
	Outputs    []string       `json:"outputs"`
}

func createReport(execScheduler *execution.Scheduler, importedModules []string, outputs []string) report {
	executors := make(map[string]int)
	for _, ec := range execScheduler.GetExecutorConfigs() {
		executors[ec.GetType()]++
	}

	// collect the report only with k6 public modules
	publicModules := make([]string, 0, len(importedModules))
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
		publicModules = append(publicModules, module)
	}

	builtinOutputs := builtinOutputStrings()

	// TODO: migrate to slices.Contains as soon as the k6 support
	// for Go1.20 will be over.
	builtinOutputsIndex := make(map[string]bool, len(builtinOutputs))
	for _, bo := range builtinOutputs {
		builtinOutputsIndex[bo] = true
	}

	// collect only the used outputs that are builtin
	publicOutputs := make([]string, 0, len(builtinOutputs))
	for _, o := range outputs {
		// TODO:
		// if !slices.Contains(builtinOutputs, o) {
		// 	continue
		// }
		if !builtinOutputsIndex[o] {
			continue
		}
		publicOutputs = append(publicOutputs, o)
	}

	execState := execScheduler.GetState()
	return report{
		Version:    consts.Version,
		Executors:  executors,
		VUsMax:     execState.GetInitializedVUsCount(),
		Iterations: execState.GetFullIterationCount(),
		Duration:   execState.GetCurrentTestRunDuration().String(),
		GoOS:       runtime.GOOS,
		GoArch:     runtime.GOARCH,
		Modules:    publicModules,
		Outputs:    publicOutputs,
	}
}

func reportUsage(ctx context.Context, execScheduler *execution.Scheduler, test *loadedAndConfiguredTest) error {
	outputs := make([]string, 0, len(test.derivedConfig.Out))
	for _, o := range test.derivedConfig.Out {
		outputName, _ := parseOutputArgument(o)
		outputs = append(outputs, outputName)
	}

	r := createReport(execScheduler, test.moduleResolver.Imported(), outputs)
	body, err := json.Marshal(r)
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
