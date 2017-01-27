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
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"os"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"time"
	"strings"
)

var (
	MetricReqs          = stats.New("http_reqs", stats.Counter)
	MetricReqDuration   = stats.New("http_req_duration", stats.Trend, stats.Time)
	MetricReqBlocked    = stats.New("http_req_blocked", stats.Trend, stats.Time)
	MetricReqLookingUp  = stats.New("http_req_looking_up", stats.Trend, stats.Time)
	MetricReqConnecting = stats.New("http_req_connecting", stats.Trend, stats.Time)
	MetricReqSending    = stats.New("http_req_sending", stats.Trend, stats.Time)
	MetricReqWaiting    = stats.New("http_req_waiting", stats.Trend, stats.Time)
	MetricReqReceiving  = stats.New("http_req_receiving", stats.Trend, stats.Time)
)

type Runner struct {
	URL       *url.URL
	Transport *http.Transport
	Options   lib.Options

	defaultGroup *lib.Group
}

func New(rawurl string) (*Runner, error) {
	if rawurl == "-" {
		urlbytes, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		rawurl = string(urlbytes)
	}
	u, err := url.Parse(strings.TrimSpace(rawurl))
	if err != nil {
		return nil, err
	}

	return &Runner{
		URL: u,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 60 * time.Second,
				DualStack: true,
			}).DialContext,
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
	resp, err := u.Client.Do(u.Request.WithContext(httptrace.WithClientTrace(ctx, u.cTrace)))
	if err != nil {
		u.tracer.Done()
		return nil, err
	}

	_, _ = io.Copy(ioutil.Discard, resp.Body)
	_ = resp.Body.Close()
	trail := u.tracer.Done()

	tags := map[string]string{
		"vu":     u.IDString,
		"method": "GET",
		"url":    u.URLString,
		"status": strconv.Itoa(resp.StatusCode),
	}

	t := time.Now()
	return []stats.Sample{
		{Metric: MetricReqs, Time: t, Tags: tags, Value: 1},
		{Metric: MetricReqDuration, Time: t, Tags: tags, Value: float64(trail.Duration)},
		{Metric: MetricReqBlocked, Time: t, Tags: tags, Value: float64(trail.Blocked)},
		{Metric: MetricReqLookingUp, Time: t, Tags: tags, Value: float64(trail.LookingUp)},
		{Metric: MetricReqConnecting, Time: t, Tags: tags, Value: float64(trail.Connecting)},
		{Metric: MetricReqSending, Time: t, Tags: tags, Value: float64(trail.Sending)},
		{Metric: MetricReqWaiting, Time: t, Tags: tags, Value: float64(trail.Waiting)},
		{Metric: MetricReqReceiving, Time: t, Tags: tags, Value: float64(trail.Receiving)},
	}, nil
}

func (u *VU) Reconfigure(id int64) error {
	u.ID = id
	u.IDString = strconv.FormatInt(id, 10)
	return nil
}
