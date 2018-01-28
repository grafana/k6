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
	"container/ring"
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"time"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/stats"
	"github.com/oxtoacart/bpool"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/viki-org/dnscache"
	"golang.org/x/net/http2"
	"golang.org/x/time/rate"
)

var errInterrupt = errors.New("context cancelled")

type Runner struct {
	Bundle       *Bundle
	Logger       *log.Logger
	defaultGroup *lib.Group

	BaseDialer net.Dialer
	Resolver   *dnscache.Resolver
	RPSLimit   *rate.Limiter
}

func New(src *lib.SourceData, fs afero.Fs) (*Runner, error) {
	bundle, err := NewBundle(src, fs)
	if err != nil {
		return nil, err
	}
	return NewFromBundle(bundle)
}

func NewFromArchive(arc *lib.Archive) (*Runner, error) {
	bundle, err := NewBundleFromArchive(arc)
	if err != nil {
		return nil, err
	}
	return NewFromBundle(bundle)
}

func NewFromBundle(b *Bundle) (*Runner, error) {
	defaultGroup, err := lib.NewGroup("", nil)
	if err != nil {
		return nil, err
	}

	r := &Runner{
		Bundle:       b,
		Logger:       log.StandardLogger(),
		defaultGroup: defaultGroup,
		BaseDialer: net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		},
		Resolver: dnscache.New(0),
	}
	r.SetOptions(r.Bundle.Options)
	return r, nil
}

func (r *Runner) MakeArchive() *lib.Archive {
	return r.Bundle.MakeArchive()
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

	var cipherSuites []uint16
	if r.Bundle.Options.TLSCipherSuites != nil {
		cipherSuites = *r.Bundle.Options.TLSCipherSuites
	}

	var tlsVersions lib.TLSVersions
	if r.Bundle.Options.TLSVersion != nil {
		tlsVersions = *r.Bundle.Options.TLSVersion
	}

	tlsAuth := r.Bundle.Options.TLSAuth
	certs := make([]tls.Certificate, len(tlsAuth))
	nameToCert := make(map[string]*tls.Certificate)
	for i, auth := range tlsAuth {
		for _, name := range auth.Domains {
			cert, err := auth.Certificate()
			if err != nil {
				return nil, err
			}
			certs[i] = *cert
			nameToCert[name] = &certs[i]
		}
	}

	hosts := map[string]*ring.Ring{}
	for host, ips := range r.Bundle.Options.Hosts {
		hosts[host] = ring.New(len(ips))

		for i := 0; i < len(ips); i++ {
			hosts[host].Value = ips[i]
			hosts[host] = hosts[host].Next()
		}
	}

	dialer := &netext.Dialer{
		Dialer:    r.BaseDialer,
		Resolver:  r.Resolver,
		Blacklist: r.Bundle.Options.BlacklistIPs,
		Hosts:     hosts,
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: r.Bundle.Options.InsecureSkipTLSVerify.Bool,
			CipherSuites:       cipherSuites,
			MinVersion:         uint16(tlsVersions.Min),
			MaxVersion:         uint16(tlsVersions.Max),
			Certificates:       certs,
			NameToCertificate:  nameToCert,
			Renegotiation:      tls.RenegotiateFreelyAsClient,
		},
		DialContext:        dialer.DialContext,
		DisableCompression: true,
	}
	_ = http2.ConfigureTransport(transport)

	vu := &VU{
		BundleInstance: *bi,
		Runner:         r,
		HTTPTransport:  transport,
		Dialer:         dialer,
		Console:        NewConsole(),
		BPool:          bpool.NewBufferPool(100),
	}
	vu.Runtime.Set("console", common.Bind(vu.Runtime, vu.Console, vu.Context))

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

func (r *Runner) SetOptions(opts lib.Options) {
	r.Bundle.Options = opts

	r.RPSLimit = nil
	if rps := opts.RPS; rps.Valid {
		r.RPSLimit = rate.NewLimiter(rate.Limit(rps.Int64), 1)
	}
}

type VU struct {
	BundleInstance

	Runner        *Runner
	HTTPTransport *http.Transport
	Dialer        *netext.Dialer
	ID            int64
	Iteration     int64

	Console *Console
	BPool   *bpool.BufferPool

	// A VU will track the last context it was called with for cancellation.
	// Note that interruptTrackedCtx is the context that is currently being tracked, while
	// interruptCancel cancels an unrelated context that terminates the tracking goroutine
	// without triggering an interrupt (for if the context changes).
	// There are cleaner ways of handling the interruption problem, but this is a hot path that
	// needs to be called thousands of times per second, which rules out anything that spawns a
	// goroutine per call.
	interruptTrackedCtx context.Context
	interruptCancel     context.CancelFunc
}

func (u *VU) RunOnce(ctx context.Context) ([]stats.Sample, error) {
	// Track the context and interrupt JS execution if it's cancelled.
	if u.interruptTrackedCtx != ctx {
		interCtx, interCancel := context.WithCancel(context.Background())
		if u.interruptCancel != nil {
			u.interruptCancel()
		}
		u.interruptCancel = interCancel
		u.interruptTrackedCtx = ctx
		go func() {
			select {
			case <-interCtx.Done():
			case <-ctx.Done():
				u.Runtime.Interrupt(errInterrupt)
			}
		}()
	}

	cookieJar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	state := &common.State{
		Logger:        u.Runner.Logger,
		Options:       u.Runner.Bundle.Options,
		Group:         u.Runner.defaultGroup,
		HTTPTransport: u.HTTPTransport,
		Dialer:        u.Dialer,
		CookieJar:     cookieJar,
		RPSLimit:      u.Runner.RPSLimit,
		BPool:         u.BPool,
		Vu:            u.ID,
		Iteration:     u.Iteration,
	}
	u.Dialer.BytesRead = &state.BytesRead
	u.Dialer.BytesWritten = &state.BytesWritten

	ctx = common.WithRuntime(ctx, u.Runtime)
	ctx = common.WithState(ctx, state)
	*u.Context = ctx

	u.Runtime.Set("__ITER", u.Iteration)
	iter := u.Iteration
	u.Iteration++

	startTime := time.Now()
	_, err = u.Default(goja.Undefined())

	t := time.Now()
	tags := map[string]string{
		"vu":   strconv.FormatInt(u.ID, 10),
		"iter": strconv.FormatInt(iter, 10)}

	samples := append(state.Samples,
		stats.Sample{Time: t, Metric: metrics.DataSent, Value: float64(state.BytesWritten), Tags: tags},
		stats.Sample{Time: t, Metric: metrics.DataReceived, Value: float64(state.BytesRead), Tags: tags},
		stats.Sample{Time: t, Metric: metrics.IterationDuration, Value: stats.D(t.Sub(startTime)), Tags: tags},
	)

	if u.Runner.Bundle.Options.NoConnectionReuse.Bool {
		u.HTTPTransport.CloseIdleConnections()
	}
	return samples, err
}

func (u *VU) Reconfigure(id int64) error {
	u.ID = id
	u.Iteration = 0
	u.Runtime.Set("__VU", u.ID)
	return nil
}
