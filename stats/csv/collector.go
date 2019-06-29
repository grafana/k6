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
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

// Collector saving output to csv implements the lib.Collector interface
type Collector struct {
	outfile io.WriteCloser
	fname   string
	resTags []string
}

// Verify that Collector implements lib.Collector
var _ lib.Collector = &Collector{}

// Similar to ioutil.NopCloser, but for writers
type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

// New Creates new instance of CSV collector
func New(fs afero.Fs, fname string, tags lib.TagSet) (*Collector, error) {
	if fname == "" || fname == "-" {
		return &Collector{
			outfile: nopCloser{os.Stdout},
			fname:   "-",
		}, nil
	}

	logfile, err := fs.Create(fname)
	if err != nil {
		return nil, err
	}

	resTags := []string{}
	for tag, flag := range tags {
		if flag {
			resTags = append(resTags, tag)
		}
	}

	return &Collector{
		outfile: logfile,
		fname:   fname,
		resTags: resTags,
	}, nil
}

// Init writes column names to csv file
func (c *Collector) Init() error {
	header := MakeHeader(c.resTags)
	c.WriteToCSV(header)
	return nil
}

// SetRunStatus does nothing
func (c *Collector) SetRunStatus(status lib.RunStatus) {}

// Run just blocks until the context is done
func (c *Collector) Run(ctx context.Context) {
	log.WithField("filename", c.fname).Debug("CSV: Writing CSV metrics")
	<-ctx.Done()
	_ = c.outfile.Close()
}

// Collect Writes samples to the csv file
func (c *Collector) Collect(scs []stats.SampleContainer) {
	for _, sc := range scs {
		for _, sample := range sc.GetSamples() {
			row := SampleToRow(&sample, c.resTags)
			c.WriteToCSV(row)
		}
	}
}

// Link returns a dummy string, it's only included to satisfy the lib.Collector interface
func (c *Collector) Link() string {
	return ""
}

// WriteToCSV writes row to csv file
func (c *Collector) WriteToCSV(row []string) {
	writer := csv.NewWriter(c.outfile)
	defer writer.Flush()
	err := writer.Write(row)
	if err != nil {
		log.WithField("filename", c.fname).Error("CSV: Error writing to file")
	}
}

// MakeHeader creates list of column names for csv file
func MakeHeader(tags []string) []string {
	return append([]string{"metric_name", "timestamp", "metric_value"}, tags...)
}

// SampleToRow converts sample into array of strings
func SampleToRow(sample *stats.Sample, resTags []string) []string {
	if sample == nil {
		return nil
	}

	row := []string{}
	row = append(row, sample.Metric.Name)
	row = append(row, fmt.Sprintf("%d", sample.Time.Unix()))
	row = append(row, fmt.Sprintf("%f", sample.Value))
	sampleTags := sample.Tags.CloneTags()

	for _, tag := range resTags {
		row = append(row, sampleTags[tag])
	}

	return row
}

// GetRequiredSystemTags returns which sample tags are needed by this collector
func (c *Collector) GetRequiredSystemTags() lib.TagSet {
	return lib.TagSet{} // There are no required tags for this collector
}
