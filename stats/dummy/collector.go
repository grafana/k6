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

package dummy

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
)

// Collector implements the lib.Collector interface and should be used only for testing
type Collector struct {
	RunStatus lib.RunStatus

	SampleContainers []stats.SampleContainer
	Samples          []stats.Sample
}

// Verify that Collector implements lib.Collector
var _ lib.Collector = &Collector{}

// Init does nothing, it's only included to satisfy the lib.Collector interface
func (c *Collector) Init() error { return nil }

// MakeConfig does nothing, it's only included to satisfy the lib.Collector interface
func (c *Collector) MakeConfig() interface{} { return nil }

// Run just blocks until the context is done
func (c *Collector) Run(ctx context.Context) {
	<-ctx.Done()
	logrus.Debugf("finished status: %d", c.RunStatus)
}

// Collect just appends all of the samples passed to it to the internal sample slice.
// According to the the lib.Collector interface, it should never be called concurrently,
// so there's no locking on purpose - that way Go's race condition detector can actually
// detect incorrect usage.
// Also, theoretically the collector doesn't have to actually Run() before samples start
// being collected, it only has to be initialized.
func (c *Collector) Collect(scs []stats.SampleContainer) {
	for _, sc := range scs {
		c.SampleContainers = append(c.SampleContainers, sc)
		c.Samples = append(c.Samples, sc.GetSamples()...)
	}
}

// Link returns a dummy string, it's only included to satisfy the lib.Collector interface
func (c *Collector) Link() string {
	return "http://example.com/"
}

// GetRequiredSystemTags returns which sample tags are needed by this collector
func (c *Collector) GetRequiredSystemTags() stats.SystemTagSet {
	return stats.SystemTagSet(0) // There are no required tags for this collector
}

// SetRunStatus just saves the passed status for later inspection
func (c *Collector) SetRunStatus(status lib.RunStatus) {
	c.RunStatus = status
}
