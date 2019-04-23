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
	"errors"
	"fmt"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/types"
	"github.com/sirupsen/logrus"
	null "gopkg.in/guregu/null.v3"
)

const manualExecution = "manual-execution"

// ManualExecutionConfig stores VUs and duration
type ManualExecutionConfig struct {
	StartVUs null.Int
	MaxVUs   null.Int
	Duration types.NullDuration
}

// NewManualExecutionConfig returns a ManualExecutionConfig with default values
func NewManualExecutionConfig(startVUs, maxVUs null.Int, duration types.NullDuration) ManualExecutionConfig {
	if !maxVUs.Valid {
		maxVUs = startVUs
	}
	return ManualExecutionConfig{startVUs, maxVUs, duration}
}

// Make sure we implement the lib.SchedulerConfig interface
var _ lib.SchedulerConfig = &ManualExecutionConfig{}

// GetDescription returns a human-readable description of the scheduler options
func (mec ManualExecutionConfig) GetDescription(_ *lib.ExecutionSegment) string {
	duration := ""
	if mec.Duration.Duration != 0 {
		duration = fmt.Sprintf(" and duration %s", mec.Duration)
	}
	return fmt.Sprintf(
		"Manual execution with %d starting and %d initialized VUs%s",
		mec.StartVUs.Int64, mec.MaxVUs.Int64, duration,
	)
}

// Validate makes sure all options are configured and valid
func (mec ManualExecutionConfig) Validate() []error {
	var errors []error
	if mec.StartVUs.Int64 <= 0 {
		errors = append(errors, fmt.Errorf("the number of VUs should be more than 0"))
	}

	if mec.MaxVUs.Int64 < mec.StartVUs.Int64 {
		errors = append(errors, fmt.Errorf("the number of MaxVUs should more than or equal to the starting number of VUs"))
	}

	if !mec.Duration.Valid {
		errors = append(errors, fmt.Errorf("the duration should be specified, for infinite duration use 0"))
	} else if time.Duration(mec.Duration.Duration) < 0 {
		errors = append(errors, fmt.Errorf(
			"the duration shouldn't be negative, for infinite duration use 0",
		))
	}

	return errors
}

// GetExecutionRequirements just reserves the number of starting VUs for the whole
// duration of the scheduler, so these VUs can be initialized in the beginning of the
// test.
//
// Importantly, if 0 (i.e. infinite) duration is configured, this scheduler doesn't
// emit the last step to relinquish these VUs.
//
// Also, the manual execution scheduler doesn't set MaxUnplannedVUs in the returned steps,
// since their initialization and usage is directly controlled by the user and is effectively
// bounded only by the resources of the machine k6 is running on.
//
// This is not a problem, because the MaxUnplannedVUs are mostly meant to be used for
// calculating the maximum possble number of initialized VUs at any point during a test
// run. That's used for sizing purposes and for user qouta checking in the cloud execution,
// where the manual scheduler isn't supported.
func (mec ManualExecutionConfig) GetExecutionRequirements(es *lib.ExecutionSegment) []lib.ExecutionStep {
	startVUs := lib.ExecutionStep{
		TimeOffset:      0,
		PlannedVUs:      uint64(es.Scale(mec.StartVUs.Int64)),
		MaxUnplannedVUs: 0, // intentional, see function comment
	}

	maxDuration := time.Duration(mec.Duration.Duration)
	if maxDuration == 0 {
		// Infinite duration, don't emit 0 VUs at the end since there's no planned end
		return []lib.ExecutionStep{startVUs}
	}
	return []lib.ExecutionStep{startVUs, {
		TimeOffset:      maxDuration,
		PlannedVUs:      0,
		MaxUnplannedVUs: 0, // intentional, see function comment
	}}
}

// GetName always returns manual-execution, since this config can't be
// specified in the exported script options.
func (ManualExecutionConfig) GetName() string {
	return manualExecution
}

// GetType always returns manual-execution, since that's this special
// config's type...
func (ManualExecutionConfig) GetType() string {
	return manualExecution
}

// GetStartTime always returns 0, since the manual execution scheduler
// always starts in the beginning and is always the only scheduler.
func (ManualExecutionConfig) GetStartTime() time.Duration {
	return 0
}

// GetGracefulStop always returns 0, since we still don't support graceful
// stops or ramp downs in the manual execution mode.
//TODO: implement?
func (ManualExecutionConfig) GetGracefulStop() time.Duration {
	return 0
}

// GetEnv returns an empty map, since the manual executor doesn't support custom
// environment variables.
func (ManualExecutionConfig) GetEnv() map[string]string {
	return nil
}

// GetExec always returns nil, for now there's no way to execute custom funcions in
// the manual execution mode.
func (ManualExecutionConfig) GetExec() null.String {
	return null.NewString("", false)
}

// IsDistributable simply returns false because there's no way to reliably
// distribute the manual execution scheduler.
func (ManualExecutionConfig) IsDistributable() bool {
	return false
}

// NewScheduler creates a new ManualExecution "scheduler"
func (mec ManualExecutionConfig) NewScheduler(
	es *lib.ExecutorState, logger *logrus.Entry) (lib.Scheduler, error) {
	return nil, errors.New("not implemented 4") //TODO
}
