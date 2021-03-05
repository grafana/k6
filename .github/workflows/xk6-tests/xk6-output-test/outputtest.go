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
package outputtest

import (
	"strconv"

	"github.com/loadimpact/k6/output"
	"github.com/loadimpact/k6/stats"
	"github.com/spf13/afero"
)

func init() {
	output.RegisterExtension("outputtest", func(params output.Params) (output.Output, error) {
		return &Output{params: params}, nil
	})
}

// Output is meant to test xk6 and the output extension sub-system of k6.
type Output struct {
	params     output.Params
	calcResult float64
	outputFile afero.File
}

// Description returns a human-readable description of the output.
func (o *Output) Description() string {
	return "test output extension"
}

// Start opens the specified output file.
func (o *Output) Start() error {
	out, err := o.params.FS.Create(o.params.ConfigArgument)
	if err != nil {
		return err
	}
	o.outputFile = out

	return nil
}

// AddMetricSamples just plucks out the metric we're interested in.
func (o *Output) AddMetricSamples(sampleContainers []stats.SampleContainer) {
	for _, sc := range sampleContainers {
		for _, sample := range sc.GetSamples() {
			if sample.Metric.Name == "foos" {
				o.calcResult += sample.Value
			}
		}
	}
}

// Stop saves the dummy results and closes the file.
func (o *Output) Stop() error {
	_, err := o.outputFile.Write([]byte(strconv.FormatFloat(o.calcResult, 'f', 0, 64)))
	if err != nil {
		return err
	}
	return o.outputFile.Close()
}
