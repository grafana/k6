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
	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestCollectorRun(t *testing.T) {
	c := &Collector{}
	assert.False(t, c.Running)

	ctx, cancel := context.WithCancel(context.Background())
	go c.Run(ctx)
	time.Sleep(1 * time.Millisecond)
	assert.True(t, c.Running, "not marked as running")

	cancel()
	time.Sleep(1 * time.Millisecond)
	assert.False(t, c.Running, "not marked as stopped")
}

func TestCollectorCollect(t *testing.T) {
	c := &Collector{}
	c.Collect([]stats.Sample{{}})
	assert.Len(t, c.Samples, 1)
}
