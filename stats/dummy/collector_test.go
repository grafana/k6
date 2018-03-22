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
	"sync"
	"testing"

	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
)

func TestCollectorRun(t *testing.T) {
	var wg sync.WaitGroup
	c := &Collector{}
	ctx, cancel := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Run(ctx)
	}()
	cancel()
	wg.Wait()
}

func TestCollectorCollect(t *testing.T) {
	c := &Collector{}
	t.Run("context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() { c.Run(ctx) }()
		c.Collect([]stats.Sample{{}})
		assert.Len(t, c.Samples, 1)
	})
}
