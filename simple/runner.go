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

package simple

import (
	"context"
	"crypto/tls"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
)

type Runner struct {
	URL       *url.URL
	Transport *http.Transport
	Options   lib.Options

	defaultGroup *lib.Group
}

func New(u *url.URL) (*Runner, error) {
	return &Runner{
		URL: u,
		Transport: &http.Transport{
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
		defaultGroup: &lib.Group{},
	}, nil
}

func (r *Runner) NewVU() (lib.VU, error) {
	tracer := &lib.Tracer{}

	return &VU{
		Runner:    r,
		URLString: r.URL.String(),
		Request: &http.Request{
			Method: "GET",
			URL:    r.URL,
		},
		Client: &http.Client{
			Transport: r.Transport,
		},
		tracer: tracer,
		cTrace: tracer.Trace(),
	}, nil
}

func (r *Runner) GetDefaultGroup() *lib.Group {
	return &lib.Group{}
}

func (r Runner) GetOptions() lib.Options {
	return r.Options
}

func (r *Runner) ApplyOptions(opts lib.Options) {
	r.Options = r.Options.Apply(opts)
	r.Transport.TLSClientConfig.InsecureSkipVerify = opts.InsecureSkipTLSVerify.Bool
}

type VU struct {
	Runner   *Runner
	ID       int64
	IDString string

	URLString string
	Request   *http.Request
	Client    *http.Client

	tracer *lib.Tracer
	cTrace *httptrace.ClientTrace
}

func (u *VU) RunOnce(ctx context.Context) ([]stats.Sample, error) {
	tags := map[string]string{
		"vu":     u.IDString,
		"status": "0",
		"method": "GET",
		"url":    u.URLString,
	}

	resp, err := u.Client.Do(u.Request.WithContext(httptrace.WithClientTrace(ctx, u.cTrace)))
	if err != nil {
		return u.tracer.Done().Samples(tags), err
	}
	tags["status"] = strconv.Itoa(resp.StatusCode)

	if _, err := io.Copy(ioutil.Discard, resp.Body); err != nil {
		return u.tracer.Done().Samples(tags), err
	}
	_ = resp.Body.Close()

	return u.tracer.Done().Samples(tags), nil
}

func (u *VU) Reconfigure(id int64) error {
	u.ID = id
	u.IDString = strconv.FormatInt(id, 10)
	return nil
}
