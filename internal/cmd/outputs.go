package cmd

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/ext"
	"go.k6.io/k6/internal/output/cloud"
	"go.k6.io/k6/internal/output/influxdb"
	"go.k6.io/k6/internal/output/json"
	"go.k6.io/k6/internal/output/opentelemetry"
	"go.k6.io/k6/internal/output/prometheusrw/remotewrite"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/output"
	"go.k6.io/k6/output/csv"

	"github.com/grafana/xk6-dashboard/dashboard"
)

// builtinOutput marks the available builtin outputs.
//
// NOTE: that the enumer is the github.com/dmarkham/enumer
//
//go:generate enumer -type=builtinOutput -trimprefix builtinOutput -transform=kebab -output builtin_output_gen.go
type builtinOutput uint32

const (
	builtinOutputCloud builtinOutput = iota
	builtinOutputCSV
	builtinOutputDatadog
	builtinOutputExperimentalPrometheusRW
	builtinOutputInfluxdb
	builtinOutputJSON
	builtinOutputKafka
	builtinOutputStatsd
	builtinOutputExperimentalOpentelemetry
)

// TODO: move this to an output sub-module after we get rid of the old collectors?
func getAllOutputConstructors() (map[string]output.Constructor, error) {
	// Start with the built-in outputs
	result := map[string]output.Constructor{
		builtinOutputJSON.String():     json.New,
		builtinOutputCloud.String():    cloud.New,
		builtinOutputCSV.String():      csv.New,
		builtinOutputInfluxdb.String(): influxdb.New,
		builtinOutputKafka.String(): func(_ output.Params) (output.Output, error) {
			return nil, errors.New("the kafka output was deprecated in k6 v0.32.0 and removed in k6 v0.34.0, " +
				"please use the new xk6 kafka output extension instead - https://github.com/k6io/xk6-output-kafka")
		},
		builtinOutputStatsd.String(): func(_ output.Params) (output.Output, error) {
			return nil, errors.New("the statsd output was deprecated in k6 v0.47.0 and removed in k6 v0.55.0, " +
				"please use the new xk6 statsd output extension instead. " +
				"It can be found at https://github.com/LeonAdato/xk6-output-statsd and " +
				"more info at https://github.com/grafana/k6/issues/2982")
		},
		builtinOutputDatadog.String(): func(_ output.Params) (output.Output, error) {
			return nil, errors.New("the datadog output was deprecated in k6 v0.32.0 and removed in k6 v0.34.0, " +
				"please use the statsd output extension https://github.com/LeonAdato/xk6-output-statsd with environment " +
				"variable K6_STATSD_ENABLE_TAGS=true or an experimental opentelemetry output instead",
			)
		},
		builtinOutputExperimentalPrometheusRW.String(): func(params output.Params) (output.Output, error) {
			return remotewrite.New(params)
		},
		"web-dashboard": dashboard.New,
		builtinOutputExperimentalOpentelemetry.String(): func(params output.Params) (output.Output, error) {
			return opentelemetry.New(params)
		},
	}

	exts := ext.Get(ext.OutputExtension)
	for _, e := range exts {
		if _, ok := result[e.Name]; ok {
			return nil, fmt.Errorf("invalid output extension %s, built-in output with the same type already exists", e.Name)
		}
		m, ok := e.Module.(output.Constructor)
		if !ok {
			return nil, fmt.Errorf("unexpected output extension type %T", e.Module)
		}
		result[e.Name] = m
	}

	return result, nil
}

func getPossibleIDList(constrs map[string]output.Constructor) string {
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
	gs *state.GlobalState, test *loadedAndConfiguredTest, executionPlan []lib.ExecutionStep,
) ([]output.Output, error) {
	outputConstructors, err := getAllOutputConstructors()
	if err != nil {
		return nil, err
	}
	baseParams := output.Params{
		ScriptPath:     test.source.URL,
		Logger:         gs.Logger,
		Environment:    gs.Env,
		StdOut:         gs.Stdout,
		StdErr:         gs.Stderr,
		FS:             gs.FS,
		ScriptOptions:  test.derivedConfig.Options,
		RuntimeOptions: test.preInitState.RuntimeOptions,
		ExecutionPlan:  executionPlan,
		Usage:          test.preInitState.Usage,
	}

	outputs := test.derivedConfig.Out
	if test.derivedConfig.WebDashboard.Bool {
		outputs = append(outputs, dashboard.OutputName)
	}

	result := make([]output.Output, 0, len(outputs))

	for _, outputFullArg := range outputs {
		outputType, outputArg, _ := strings.Cut(outputFullArg, "=")
		outputConstructor, ok := outputConstructors[outputType]
		if !ok {
			return nil, fmt.Errorf(
				"invalid output type '%s', available types are: %s",
				outputType, getPossibleIDList(outputConstructors),
			)
		}
		if _, builtinErr := builtinOutputString(outputType); builtinErr == nil {
			err := test.preInitState.Usage.Strings("outputs", outputType)
			if err != nil {
				gs.Logger.WithError(err).Warnf("Couldn't report usage for output %q", outputType)
			}
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

		// If the output is configured to support the archive, and supports it, we proceed
		// with building an archive and setting it on the output instance.
		if !test.derivedConfig.NoArchiveUpload.Bool {
			if archiveOut, ok := out.(output.WithArchive); ok {
				archiveOut.SetArchive(test.initRunner.MakeArchive())
			}
		}

		result = append(result, out)
	}

	return result, nil
}
