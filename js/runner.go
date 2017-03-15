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
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/robertkrimen/otto"
)

var ErrDefaultExport = errors.New("you must export a 'default' function")

const entrypoint = "__$$entrypoint$$__"

type Runner struct {
	Runtime      *Runtime
	DefaultGroup *lib.Group
	Options      lib.Options

	HTTPTransport *http.Transport
}

func NewRunner(rt *Runtime, exports otto.Value) (*Runner, error) {
	expObj := exports.Object()
	if expObj == nil {
		return nil, ErrDefaultExport
	}

	// Values "remember" which VM they belong to, so to get a callable that works across VM copies,
	// we have to stick it in the global scope, then retrieve it again from the new instance.
	callable, err := expObj.Get("default")
	if err != nil {
		return nil, err
	}
	if !callable.IsFunction() {
		return nil, ErrDefaultExport
	}
	if err := rt.VM.Set(entrypoint, callable); err != nil {
		return nil, err
	}

	defaultGroup, err := lib.NewGroup("", nil)
	if err != nil {
		return nil, err
	}

	r := &Runner{
		Runtime:      rt,
		DefaultGroup: defaultGroup,
		Options:      rt.Options,
		HTTPTransport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 60 * time.Second,
				DualStack: true,
			}).DialContext,
			TLSClientConfig:     &tls.Config{},
			MaxIdleConns:        math.MaxInt32,
			MaxIdleConnsPerHost: math.MaxInt32,
		},
	}

	return r, nil
}

func (r *Runner) NewVU() (lib.VU, error) {
	u := &VU{
		runner: r,
		vm:     r.Runtime.VM.Copy(),
		group:  r.DefaultGroup,
	}

	u.CookieJar = lib.NewCookieJar()
	u.HTTPClient = &http.Client{
		Transport:     r.HTTPTransport,
		CheckRedirect: u.checkRedirect,
		Jar:           u.CookieJar,
	}

	callable, err := u.vm.Get(entrypoint)
	if err != nil {
		return nil, err
	}
	u.callable = callable

	if err := u.vm.Set("__jsapi__", &JSAPI{u}); err != nil {
		return nil, err
	}

	return u, nil
}

func (r *Runner) GetDefaultGroup() *lib.Group {
	return r.DefaultGroup
}

func (r *Runner) GetOptions() lib.Options {
	return r.Options
}

func (r *Runner) ApplyOptions(opts lib.Options) {
	r.Options = r.Options.Apply(opts)
	r.HTTPTransport.TLSClientConfig.InsecureSkipVerify = opts.InsecureSkipTLSVerify.Bool
}

type VU struct {
	ID        int64
	IDString  string
	Iteration int64
	Samples   []stats.Sample

	runner   *Runner
	vm       *otto.Otto
	callable otto.Value

	HTTPClient *http.Client
	CookieJar  *lib.CookieJar

	started time.Time
	ctx     context.Context
	group   *lib.Group
}

func (u *VU) RunOnce(ctx context.Context) ([]stats.Sample, error) {
	u.CookieJar.Clear()

	if err := u.vm.Set("__ITER", u.Iteration); err != nil {
		return nil, err
	}

	u.started = time.Now()
	u.ctx = ctx
	_, err := u.callable.Call(otto.UndefinedValue())
	u.ctx = nil

	u.Iteration++

	samples := u.Samples
	u.Samples = nil
	return samples, err
}

func (u *VU) Reconfigure(id int64) error {
	u.ID = id
	u.IDString = strconv.FormatInt(u.ID, 10)
	u.Iteration = 0

	if err := u.vm.Set("__VU", u.ID); err != nil {
		return err
	}
	if err := u.vm.Set("__ITER", u.Iteration); err != nil {
		return err
	}

	return nil
}

func (u *VU) checkRedirect(req *http.Request, via []*http.Request) error {
	log.WithFields(log.Fields{
		"from": via[len(via)-1].URL.String(),
		"to":   req.URL.String(),
	}).Debug("-> Redirect")
	if int64(len(via)) >= u.runner.Options.MaxRedirects.Int64 {
		return errors.New(fmt.Sprintf("stopped after %d redirects", u.runner.Options.MaxRedirects.Int64))
	}
	return nil
}
