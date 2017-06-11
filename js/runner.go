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

package js

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/stats"
	"github.com/spf13/afero"
)

type Runner struct {
	Bundle       *Bundle
	defaultGroup *lib.Group

	Dialer *netext.Dialer
}

func New(src *lib.SourceData, fs afero.Fs) (*Runner, error) {
	bundle, err := NewBundle(src, fs)
	if err != nil {
		return nil, err
	}

	defaultGroup, err := lib.NewGroup("", nil)
	if err != nil {
		return nil, err
	}

	return &Runner{
		Bundle:       bundle,
		defaultGroup: defaultGroup,
		Dialer: netext.NewDialer(net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}),
	}, nil
}

func (r *Runner) NewVU() (lib.VU, error) {
	vu, err := r.newVU()
	if err != nil {
		return nil, err
	}
	return lib.VU(vu), nil
}

func (r *Runner) newVU() (*VU, error) {
	// Instantiate a new bundle, make a VU out of it.
	bi, err := r.Bundle.Instantiate()
	if err != nil {
		return nil, err
	}

	// Make a VU, apply the VU context.
	vu := &VU{
		BundleInstance: *bi,
		Runner:         r,
		HTTPTransport:  &http.Transport{DialContext: r.Dialer.DialContext},
		VUContext:      NewVUContext(),
	}
	common.BindToGlobal(vu.Runtime, common.Bind(vu.Runtime, vu.VUContext, vu.Context))

	// Give the VU an initial sense of identity.
	if err := vu.Reconfigure(0); err != nil {
		return nil, err
	}

	return vu, nil
}

func (r *Runner) GetDefaultGroup() *lib.Group {
	return r.defaultGroup
}

func (r *Runner) GetOptions() lib.Options {
	return r.Bundle.Options
}

func (r *Runner) ApplyOptions(opts lib.Options) {
	r.Bundle.Options = r.Bundle.Options.Apply(opts)
}

type VU struct {
	BundleInstance

	Runner        *Runner
	HTTPTransport *http.Transport
	ID            int64
	Iteration     int64

	VUContext *VUContext
}

func (u *VU) RunOnce(ctx context.Context) ([]stats.Sample, error) {
	state := &common.State{
		Group:         u.Runner.defaultGroup,
		HTTPTransport: u.HTTPTransport,
		Dialer:        u.Runner.Dialer,
	}

	ctx = common.WithRuntime(ctx, u.Runtime)
	ctx = common.WithState(ctx, state)
	*u.Context = ctx

	u.Runtime.Set("__ITER", u.Iteration)
	u.Iteration++

	_, err := u.Default(goja.Undefined())

	return state.Samples, err
}

func (u *VU) Reconfigure(id int64) error {
	u.ID = id
	u.Iteration = 0
	u.Runtime.Set("__VU", u.ID)
	return nil
}
