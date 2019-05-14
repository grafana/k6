/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package scheduler

import (
	"context"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/ui/pb"
	"github.com/sirupsen/logrus"
)

// BaseScheduler is a helper struct that contains common properties and methods
// between most schedulers. It is intended to be used as an anonymous struct
// inside of most of the schedulers, for the purpose of reducing boilerplate
// code.
type BaseScheduler struct {
	config        lib.SchedulerConfig
	executorState *lib.ExecutorState
	logger        *logrus.Entry
	progress      *pb.ProgressBar
}

// NewBaseScheduler just returns an initialized BaseScheduler
func NewBaseScheduler(config lib.SchedulerConfig, es *lib.ExecutorState, logger *logrus.Entry) *BaseScheduler {
	return &BaseScheduler{
		config:        config,
		executorState: es,
		logger:        logger,
		progress: pb.New(
			pb.WithLeft(config.GetName),
		),
	}
}

// Init doesn't do anything for most schedulers, since initialization of all
// planned VUs is handled by the executor.
func (bs *BaseScheduler) Init(_ context.Context) error {
	return nil
}

// GetConfig returns the configuration with which this scheduler was launched.
func (bs BaseScheduler) GetConfig() lib.SchedulerConfig {
	return bs.config
}

// GetLogger returns the scheduler logger entry.
func (bs BaseScheduler) GetLogger() *logrus.Entry {
	return bs.logger
}

// GetProgress just returns the progressbar pointer.
func (bs BaseScheduler) GetProgress() *pb.ProgressBar {
	return bs.progress
}
