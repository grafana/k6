/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package csv

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"go.k6.io/k6/output"
	"go.k6.io/k6/stats"
)

// Output implements the lib.Output interface for saving to CSV files.
type Output struct {
	output.SampleBuffer

	params          output.Params
	periodicFlusher *output.PeriodicFlusher

	logger    logrus.FieldLogger
	fname     string
	csvWriter *csv.Writer
	csvLock   sync.Mutex
	closeFn   func() error

	resTags      []string
	ignoredTags  []string
	row          []string
	saveInterval time.Duration
}

// New Creates new instance of CSV output
func New(params output.Params) (output.Output, error) {
	return newOutput(params)
}

func newOutput(params output.Params) (*Output, error) {
	resTags := []string{}
	ignoredTags := []string{}
	tags := params.ScriptOptions.SystemTags.Map()
	for tag, flag := range tags {
		if flag {
			resTags = append(resTags, tag)
		} else {
			ignoredTags = append(ignoredTags, tag)
		}
	}

	sort.Strings(resTags)
	sort.Strings(ignoredTags)

	config, err := GetConsolidatedConfig(params.JSONConfig, params.Environment, params.ConfigArgument)
	if err != nil {
		return nil, err
	}

	saveInterval := time.Duration(config.SaveInterval.Duration)
	fname := config.FileName.String

	logger := params.Logger.WithFields(logrus.Fields{
		"output":   "csv",
		"filename": params.ConfigArgument,
	})
	if fname == "" || fname == "-" {
		stdoutWriter := csv.NewWriter(os.Stdout)
		return &Output{
			fname:        "-",
			resTags:      resTags,
			ignoredTags:  ignoredTags,
			csvWriter:    stdoutWriter,
			row:          make([]string, 3+len(resTags)+1),
			saveInterval: saveInterval,
			closeFn:      func() error { return nil },
			logger:       logger,
			params:       params,
		}, nil
	}

	logFile, err := params.FS.Create(fname)
	if err != nil {
		return nil, err
	}

	c := Output{
		fname:        fname,
		resTags:      resTags,
		ignoredTags:  ignoredTags,
		row:          make([]string, 3+len(resTags)+1),
		saveInterval: saveInterval,
		logger:       logger,
		params:       params,
	}

	if strings.HasSuffix(fname, ".gz") {
		outfile := gzip.NewWriter(logFile)
		csvWriter := csv.NewWriter(outfile)
		c.csvWriter = csvWriter
		c.closeFn = func() error {
			_ = outfile.Close()
			return logFile.Close()
		}
	} else {
		csvWriter := csv.NewWriter(logFile)
		c.csvWriter = csvWriter
		c.closeFn = logFile.Close
	}

	return &c, nil
}

// Description returns a human-readable description of the output.
func (o *Output) Description() string {
	if o.fname == "" || o.fname == "-" { // TODO rename
		return "csv (stdout)"
	}
	return fmt.Sprintf("csv (%s)", o.fname)
}

// Start writes the csv header and starts a new output.PeriodicFlusher
func (o *Output) Start() error {
	o.logger.Debug("Starting...")

	header := MakeHeader(o.resTags)
	err := o.csvWriter.Write(header)
	if err != nil {
		o.logger.WithField("filename", o.fname).Error("CSV: Error writing column names to file")
	}
	o.csvWriter.Flush()

	pf, err := output.NewPeriodicFlusher(o.saveInterval, o.flushMetrics)
	if err != nil {
		return err
	}
	o.logger.Debug("Started!")
	o.periodicFlusher = pf

	return nil
}

// Stop flushes any remaining metrics and stops the goroutine.
func (o *Output) Stop() error {
	o.logger.Debug("Stopping...")
	defer o.logger.Debug("Stopped!")
	o.periodicFlusher.Stop()
	return o.closeFn()
}

// flushMetrics Writes samples to the csv file
func (o *Output) flushMetrics() {
	samples := o.GetBufferedSamples()

	if len(samples) > 0 {
		o.csvLock.Lock()
		defer o.csvLock.Unlock()
		for _, sc := range samples {
			for _, sample := range sc.GetSamples() {
				sample := sample
				row := SampleToRow(&sample, o.resTags, o.ignoredTags, o.row)
				err := o.csvWriter.Write(row)
				if err != nil {
					o.logger.WithField("filename", o.fname).Error("CSV: Error writing to file")
				}
			}
		}
		o.csvWriter.Flush()
	}
}

// MakeHeader creates list of column names for csv file
func MakeHeader(tags []string) []string {
	tags = append(tags, "extra_tags")
	return append([]string{"metric_name", "timestamp", "metric_value"}, tags...)
}

// SampleToRow converts sample into array of strings
func SampleToRow(sample *stats.Sample, resTags []string, ignoredTags []string, row []string) []string {
	row[0] = sample.Metric.Name
	row[1] = fmt.Sprintf("%d", sample.Time.Unix())
	row[2] = fmt.Sprintf("%f", sample.Value)
	sampleTags := sample.Tags.CloneTags()

	for ind, tag := range resTags {
		row[ind+3] = sampleTags[tag]
	}

	extraTags := bytes.Buffer{}
	prev := false
	for tag, val := range sampleTags {
		if !IsStringInSlice(resTags, tag) && !IsStringInSlice(ignoredTags, tag) {
			if prev {
				if _, err := extraTags.WriteString("&"); err != nil {
					break
				}
			}

			if _, err := extraTags.WriteString(tag); err != nil {
				break
			}

			if _, err := extraTags.WriteString("="); err != nil {
				break
			}

			if _, err := extraTags.WriteString(val); err != nil {
				break
			}
			prev = true
		}
	}
	row[len(row)-1] = extraTags.String()

	return row
}

// IsStringInSlice returns whether the string is contained within a string slice
func IsStringInSlice(slice []string, str string) bool {
	if index := sort.SearchStrings(slice, str); index == len(slice) || slice[index] != str {
		return false
	}
	return true
}
