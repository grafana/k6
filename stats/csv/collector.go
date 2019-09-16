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
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

// Collector saving output to csv implements the lib.Collector interface
type Collector struct {
	outfile      io.WriteCloser
	fname        string
	resTags      []string
	ignoredTags  []string
	csvWriter    *csv.Writer
	csvLock      sync.Mutex
	buffer       []stats.Sample
	bufferLock   sync.Mutex
	row          []string
	saveInterval time.Duration
}

// Verify that Collector implements lib.Collector
var _ lib.Collector = &Collector{}

// Similar to ioutil.NopCloser, but for writers
type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

// New Creates new instance of CSV collector
func New(fs afero.Fs, tags stats.SystemTagMap, config Config) (*Collector, error) {
	resTags := []string{}
	ignoredTags := []string{}
	for tag, flag := range tags {
		if flag {
			resTags = append(resTags, tag)
		} else {
			ignoredTags = append(ignoredTags, tag)
		}
	}
	sort.Strings(resTags)
	sort.Strings(ignoredTags)

	saveInterval := time.Duration(config.SaveInterval.Duration)
	fname := config.FileName.String

	if fname == "" || fname == "-" {
		logfile := nopCloser{os.Stdout}
		return &Collector{
			outfile:      logfile,
			fname:        "-",
			resTags:      resTags,
			ignoredTags:  ignoredTags,
			csvWriter:    csv.NewWriter(logfile),
			row:          make([]string, 3+len(resTags)+1),
			saveInterval: saveInterval,
		}, nil
	}

	logfile, err := fs.Create(fname)
	if err != nil {
		return nil, err
	}

	return &Collector{
		outfile:      logfile,
		fname:        fname,
		resTags:      resTags,
		ignoredTags:  ignoredTags,
		csvWriter:    csv.NewWriter(logfile),
		row:          make([]string, 3+len(resTags)+1),
		saveInterval: saveInterval,
	}, nil
}

// Init writes column names to csv file
func (c *Collector) Init() error {
	header := MakeHeader(c.resTags)
	err := c.csvWriter.Write(header)
	if err != nil {
		logrus.WithField("filename", c.fname).Error("CSV: Error writing column names to file")
	}
	c.csvWriter.Flush()
	return nil
}

// SetRunStatus does nothing
func (c *Collector) SetRunStatus(status lib.RunStatus) {}

// Run just blocks until the context is done
func (c *Collector) Run(ctx context.Context) {
	ticker := time.NewTicker(c.saveInterval)
	for {
		select {
		case <-ticker.C:
			c.WriteToFile()
		case <-ctx.Done():
			c.WriteToFile()
			err := c.outfile.Close()
			if err != nil {
				logrus.WithField("filename", c.fname).Error("CSV: Error closing the file")
			}
			return
		}
	}
}

// Collect Saves samples to buffer
func (c *Collector) Collect(scs []stats.SampleContainer) {
	c.bufferLock.Lock()
	defer c.bufferLock.Unlock()
	for _, sc := range scs {
		c.buffer = append(c.buffer, sc.GetSamples()...)
	}
}

// WriteToFile Writes samples to the csv file
func (c *Collector) WriteToFile() {
	c.bufferLock.Lock()
	samples := c.buffer
	c.buffer = nil
	c.bufferLock.Unlock()

	if len(samples) > 0 {
		c.csvLock.Lock()
		defer c.csvLock.Unlock()
		for _, sc := range samples {
			for _, sample := range sc.GetSamples() {
				sample := sample
				row := SampleToRow(&sample, c.resTags, c.ignoredTags, c.row)
				err := c.csvWriter.Write(row)
				if err != nil {
					logrus.WithField("filename", c.fname).Error("CSV: Error writing to file")
				}
			}
		}
		c.csvWriter.Flush()
	}
}

// Link returns a dummy string, it's only included to satisfy the lib.Collector interface
func (c *Collector) Link() string {
	return c.fname
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

// GetRequiredSystemTags returns which sample tags are needed by this collector
func (c *Collector) GetRequiredSystemTags() stats.SystemTagSet {
	return stats.SystemTagSet(0) // There are no required tags for this collector
}
