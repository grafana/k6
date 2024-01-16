// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only

package dashboard

import (
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/output"
)

type process struct {
	logger logrus.FieldLogger
	fs     fsext.Fs
	env    map[string]string
}

func (proc *process) fromParams(params output.Params) *process {
	proc.fs = params.FS
	proc.logger = params.Logger
	proc.env = params.Environment

	return proc
}

func (proc *process) fromGlobalState(gs *state.GlobalState) *process {
	proc.fs = gs.FS
	proc.logger = gs.Logger
	proc.env = gs.Env

	return proc
}
