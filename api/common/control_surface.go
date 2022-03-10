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

package common

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/execution"
	"go.k6.io/k6/metrics/engine"
	"go.k6.io/k6/stats"
)

// ControlSurface includes the methods the REST API can use to control and
// communicate with the rest of k6.
type ControlSurface struct {
	RunCtx             context.Context
	Samples            chan stats.SampleContainer
	MetricsEngine      *engine.MetricsEngine
	ExecutionScheduler *execution.Scheduler
	Logger             logrus.FieldLogger
}
