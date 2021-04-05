/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/output"
	"github.com/loadimpact/k6/output/cloud"
	"github.com/loadimpact/k6/output/csv"
	"github.com/loadimpact/k6/output/influxdb"
	"github.com/loadimpact/k6/output/json"
	"github.com/loadimpact/k6/output/statsd"
	"github.com/loadimpact/k6/stats"

	"github.com/k6io/xk6-output-kafka/pkg/kafka"
)

// TODO: move this to an output sub-module after we get rid of the old collectors?
//nolint: funlen
func getAllOutputConstructors() (map[string]func(output.Params) (output.Output, error), error) {
	// Start with the built-in outputs
	result := map[string]func(output.Params) (output.Output, error){
		"json":     json.New,
		"cloud":    cloud.New,
		"influxdb": influxdb.New,
		"kafka": func(params output.Params) (output.Output, error) {
			params.Logger.Warn("The kafka output is deprecated, and will be removed in a future k6 version. " +
				"Please use the new xk6 kafka output extension instead. " +
				"It can be found at https://github.com/k6io/xk6-output-kafka.")
			return kafka.New(params)
		},
		"statsd": statsd.New,
		"datadog": func(params output.Params) (output.Output, error) {
			params.Logger.Warn("The datadog output is deprecated, and will be removed in a future k6 version. " +
				"Please use the statsd output with env variable K6_STATSD_ENABLE_TAGS=true instead.")
			return statsd.NewDatadog(params)
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
		res = append(res, k)
	}
	sort.Strings(res)
	return strings.Join(res, ", ")
}

func createOutputs(
	outputFullArguments []string, src *loader.SourceData, conf Config, rtOpts lib.RuntimeOptions,
	executionPlan []lib.ExecutionStep, osEnvironment map[string]string, logger logrus.FieldLogger,
) ([]output.Output, error) {
	outputConstructors, err := getAllOutputConstructors()
	if err != nil {
		return nil, err
	}
	baseParams := output.Params{
		ScriptPath:     src.URL,
		Logger:         logger,
		Environment:    osEnvironment,
		StdOut:         stdout,
		StdErr:         stderr,
		FS:             afero.NewOsFs(),
		ScriptOptions:  conf.Options,
		RuntimeOptions: rtOpts,
		ExecutionPlan:  executionPlan,
	}
	result := make([]output.Output, 0, len(outputFullArguments))

	for _, outputFullArg := range outputFullArguments {
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
		params.JSONConfig = conf.Collectors[outputType]

		output, err := outputConstructor(params)
		if err != nil {
			return nil, fmt.Errorf("could not create the '%s' output: %w", outputType, err)
		}
		result = append(result, output)
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

// TODO: remove this after we transition every collector to the output interface

func newCollectorAdapter(params output.Params, collector lib.Collector) output.Output {
	return &collectorAdapter{
		outputType: params.OutputType,
		collector:  collector,
		stopCh:     make(chan struct{}),
	}
}

// collectorAdapter is a _temporary_ fix until we move all of the old
// "collectors" to the new output interface
type collectorAdapter struct {
	collector    lib.Collector
	outputType   string
	runCtx       context.Context
	runCtxCancel func()
	stopCh       chan struct{}
}

func (ca *collectorAdapter) Description() string {
	link := ca.collector.Link()
	if link != "" {
		return fmt.Sprintf("%s (%s)", ca.outputType, link)
	}
	return ca.outputType
}

func (ca *collectorAdapter) Start() error {
	if err := ca.collector.Init(); err != nil {
		return err
	}
	ca.runCtx, ca.runCtxCancel = context.WithCancel(context.Background())
	go func() {
		ca.collector.Run(ca.runCtx)
		close(ca.stopCh)
	}()
	return nil
}

func (ca *collectorAdapter) AddMetricSamples(samples []stats.SampleContainer) {
	ca.collector.Collect(samples)
}

// Stop implements the new output interface.
func (ca *collectorAdapter) Stop() error {
	ca.runCtxCancel()
	<-ca.stopCh
	return nil
}
