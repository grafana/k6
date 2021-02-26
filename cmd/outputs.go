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

	"github.com/loadimpact/k6/cloudapi"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/consts"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/output"
	"github.com/loadimpact/k6/output/json"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/cloud"
	"github.com/loadimpact/k6/stats/csv"
	"github.com/loadimpact/k6/stats/datadog"
	"github.com/loadimpact/k6/stats/influxdb"
	"github.com/loadimpact/k6/stats/kafka"
	"github.com/loadimpact/k6/stats/statsd"
)

// TODO: move this to an output sub-module after we get rid of the old collectors?
//nolint: funlen
func getAllOutputConstructors() (map[string]func(output.Params) (output.Output, error), error) {
	// Start with the built-in outputs
	result := map[string]func(output.Params) (output.Output, error){
		"json": json.New,

		// TODO: remove all of these
		"influxdb": func(params output.Params) (output.Output, error) {
			conf, err := influxdb.GetConsolidatedConfig(params.JSONConfig, params.Environment, params.ConfigArgument)
			if err != nil {
				return nil, err
			}
			influxc, err := influxdb.New(params.Logger, conf)
			if err != nil {
				return nil, err
			}
			return newCollectorAdapter(params, influxc)
		},
		"cloud": func(params output.Params) (output.Output, error) {
			conf, err := cloudapi.GetConsolidatedConfig(params.JSONConfig, params.Environment, params.ConfigArgument)
			if err != nil {
				return nil, err
			}
			cloudc, err := cloud.New(
				params.Logger, conf, params.ScriptPath, params.ScriptOptions, params.ExecutionPlan, consts.Version,
			)
			if err != nil {
				return nil, err
			}
			return newCollectorAdapter(params, cloudc)
		},
		"kafka": func(params output.Params) (output.Output, error) {
			conf, err := kafka.GetConsolidatedConfig(params.JSONConfig, params.Environment, params.ConfigArgument)
			if err != nil {
				return nil, err
			}
			kafkac, err := kafka.New(params.Logger, conf)
			if err != nil {
				return nil, err
			}
			return newCollectorAdapter(params, kafkac)
		},
		"statsd": func(params output.Params) (output.Output, error) {
			conf, err := statsd.GetConsolidatedConfig(params.JSONConfig, params.Environment)
			if err != nil {
				return nil, err
			}
			statsdc, err := statsd.New(params.Logger, conf)
			if err != nil {
				return nil, err
			}
			return newCollectorAdapter(params, statsdc)
		},
		"datadog": func(params output.Params) (output.Output, error) {
			conf, err := datadog.GetConsolidatedConfig(params.JSONConfig, params.Environment)
			if err != nil {
				return nil, err
			}
			datadogc, err := datadog.New(params.Logger, conf)
			if err != nil {
				return nil, err
			}
			return newCollectorAdapter(params, datadogc)
		},
		"csv": func(params output.Params) (output.Output, error) {
			conf, err := csv.GetConsolidatedConfig(params.JSONConfig, params.Environment, params.ConfigArgument)
			if err != nil {
				return nil, err
			}
			csvc, err := csv.New(params.Logger, params.FS, params.ScriptOptions.SystemTags.Map(), conf)
			if err != nil {
				return nil, err
			}
			return newCollectorAdapter(params, csvc)
		},
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

func newCollectorAdapter(params output.Params, collector lib.Collector) (output.Output, error) {
	// Check if all required tags are present
	missingRequiredTags := []string{}
	requiredTags := collector.GetRequiredSystemTags()
	for _, tag := range stats.SystemTagSetValues() {
		if requiredTags.Has(tag) && !params.ScriptOptions.SystemTags.Has(tag) {
			missingRequiredTags = append(missingRequiredTags, tag.String())
		}
	}
	if len(missingRequiredTags) > 0 {
		return nil, fmt.Errorf(
			"the specified output '%s' needs the following system tags enabled: %s",
			params.OutputType, strings.Join(missingRequiredTags, ", "),
		)
	}

	return &collectorAdapter{
		outputType: params.OutputType,
		collector:  collector,
		stopCh:     make(chan struct{}),
	}, nil
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

func (ca *collectorAdapter) SetRunStatus(latestStatus lib.RunStatus) {
	ca.collector.SetRunStatus(latestStatus)
}

// Stop implements the new output interface.
func (ca *collectorAdapter) Stop() error {
	ca.runCtxCancel()
	<-ca.stopCh
	return nil
}

var _ output.WithRunStatusUpdates = &collectorAdapter{}
