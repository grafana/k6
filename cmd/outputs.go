package cmd

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/output"
	"go.k6.io/k6/output/cloud"
	"go.k6.io/k6/output/csv"
	"go.k6.io/k6/output/influxdb"
	"go.k6.io/k6/output/json"
	"go.k6.io/k6/output/statsd"
)

// TODO: move this to an output sub-module after we get rid of the old collectors?
func getAllOutputConstructors() (map[string]func(output.Params) (output.Output, error), error) {
	// Start with the built-in outputs
	result := map[string]func(output.Params) (output.Output, error){
		"json":     json.New,
		"cloud":    cloud.New,
		"influxdb": influxdb.New,
		"kafka": func(params output.Params) (output.Output, error) {
			return nil, errors.New("the kafka output was deprecated in k6 v0.32.0 and removed in k6 v0.34.0, " +
				"please use the new xk6 kafka output extension instead - https://github.com/k6io/xk6-output-kafka")
		},
		"statsd": statsd.New,
		"datadog": func(params output.Params) (output.Output, error) {
			return nil, errors.New("the datadog output was deprecated in k6 v0.32.0 and removed in k6 v0.34.0, " +
				"please use the statsd output with env. variable K6_STATSD_ENABLE_TAGS=true instead")
		},
		"csv": csv.New,
	}

	exts := output.GetExtensions()
	for k, v := range exts {
		if _, ok := result[k]; ok {
			return nil, fmt.Errorf("invalid output extension %s, built-in output with the same type already exists", k)
		}
		result[k] = v
	}

	return result, nil
}

func getPossibleIDList(constrs map[string]func(output.Params) (output.Output, error)) string {
	res := make([]string, 0, len(constrs))
	for k := range constrs {
		if k == "kafka" || k == "datadog" {
			continue
		}
		res = append(res, k)
	}
	sort.Strings(res)
	return strings.Join(res, ", ")
}

func createOutputs(
	gs *globalState, test *loadedAndConfiguredTest, executionPlan []lib.ExecutionStep,
) ([]output.Output, error) {
	outputConstructors, err := getAllOutputConstructors()
	if err != nil {
		return nil, err
	}
	baseParams := output.Params{
		ScriptPath:     test.source.URL,
		Logger:         gs.logger,
		Environment:    gs.envVars,
		StdOut:         gs.stdOut,
		StdErr:         gs.stdErr,
		FS:             gs.fs,
		ScriptOptions:  test.derivedConfig.Options,
		RuntimeOptions: test.preInitState.RuntimeOptions,
		ExecutionPlan:  executionPlan,
	}
	result := make([]output.Output, 0, len(test.derivedConfig.Out))

	for _, outputFullArg := range test.derivedConfig.Out {
		outputType, outputArg := parseOutputArgument(outputFullArg)
		outputConstructor, ok := outputConstructors[outputType]
		if !ok {
			return nil, fmt.Errorf(
				"invalid output type '%s', available types are: %s",
				outputType, getPossibleIDList(outputConstructors),
			)
		}

		params := baseParams
		params.OutputType = outputType
		params.ConfigArgument = outputArg
		params.JSONConfig = test.derivedConfig.Collectors[outputType]

		out, err := outputConstructor(params)
		if err != nil {
			return nil, fmt.Errorf("could not create the '%s' output: %w", outputType, err)
		}

		if thresholdOut, ok := out.(output.WithThresholds); ok {
			thresholdOut.SetThresholds(test.derivedConfig.Thresholds)
		}

		if builtinMetricOut, ok := out.(output.WithBuiltinMetrics); ok {
			builtinMetricOut.SetBuiltinMetrics(test.preInitState.BuiltinMetrics)
		}

		result = append(result, out)
	}

	return result, nil
}

func parseOutputArgument(s string) (t, arg string) {
	parts := strings.SplitN(s, "=", 2)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	default:
		return parts[0], parts[1]
	}
}
