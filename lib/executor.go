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

package lib

import (
	"context"
	"time"

	"github.com/loadimpact/k6/stats"
	log "github.com/sirupsen/logrus"
	null "gopkg.in/guregu/null.v3"
)

// An Executor wraps a Runner, and abstracts away an execution environment.
type Executor interface {
	Run(ctx context.Context, out chan<- []stats.Sample) error
	IsRunning() bool

	GetRunner() Runner

	SetLogger(l *log.Logger)
	GetLogger() *log.Logger

	GetIterations() int64
	GetEndIterations() null.Int
	SetEndIterations(i null.Int)

	GetTime() time.Duration
	GetEndTime() NullDuration
	SetEndTime(t NullDuration)

	IsPaused() bool
	SetPaused(paused bool)

	GetVUs() int64
	SetVUs(vus int64) error

	GetVUsMax() int64
	SetVUsMax(max int64) error
}
