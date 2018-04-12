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
	"encoding/json"
	"net"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"sync/atomic"
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

	setupData interface{}
}

func New(src *lib.SourceData, fs afero.Fs, rtOpts lib.RuntimeOptions) (*Runner, error) {
	bundle, err := NewBundle(src, fs, rtOpts)
	if err != nil {
		return nil, err
	}
	return NewFromBundle(bundle)
}

func NewFromArchive(arc *lib.Archive, rtOpts lib.RuntimeOptions) (*Runner, error) {
	bundle, err := NewBundleFromArchive(arc, rtOpts)
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

	dialer := &netext.Dialer{
		Dialer:    r.BaseDialer,
		Resolver:  r.Resolver,
		Blacklist: r.Bundle.Options.BlacklistIPs,
		Hosts:     r.Bundle.Options.Hosts,
	}
	tlsConfig := &tls.Config{
		InsecureSkipVerify: r.Bundle.Options.InsecureSkipTLSVerify.Bool,
		CipherSuites:       cipherSuites,
		MinVersion:         uint16(tlsVersions.Min),
		MaxVersion:         uint16(tlsVersions.Max),
		Certificates:       certs,
		NameToCertificate:  nameToCert,
		Renegotiation:      tls.RenegotiateFreelyAsClient,
	}
	transport := &http.Transport{
		Proxy:              http.ProxyFromEnvironment,
		TLSClientConfig:    tlsConfig,
		DialContext:        dialer.DialContext,
		DisableCompression: true,
	}
	_ = http2.ConfigureTransport(transport)

	vu := &VU{
		BundleInstance: *bi,
		Runner:         r,
		HTTPTransport:  netext.NewHTTPTransport(transport),
		Dialer:         dialer,
		TLSConfig:      tlsConfig,
		Console:        NewConsole(),
		BPool:          bpool.NewBufferPool(100),
	}
	vu.Runtime.Set("console", common.Bind(vu.Runtime, vu.Console, vu.Context))
	common.BindToGlobal(vu.Runtime, map[string]interface{}{
		"open": func() {
			common.Throw(vu.Runtime, errors.New("\"open\" function is only available to the init code (aka global scope), see https://docs.k6.io/docs/test-life-cycle for more information"))
		},
	})

	// Give the VU an initial sense of identity.
	if err := vu.Reconfigure(0); err != nil {
		return nil, err
	}

	return vu, nil
}

func (r *Runner) Setup(ctx context.Context) error {
	v, err := r.runPart(ctx, "setup", nil)
	if err != nil {
		return errors.Wrap(err, "setup")
	}
	data, err := json.Marshal(v.Export())
	if err != nil {
		return errors.Wrap(err, "setup")
	}
	return json.Unmarshal(data, &r.setupData)
}

func (r *Runner) Teardown(ctx context.Context) error {
	_, err := r.runPart(ctx, "teardown", r.setupData)
	return err
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

// Runs an exported function in its own temporary VU, optionally with an argument. Execution is
// interrupted if the context expires. No error is returned if the part does not exist.
func (r *Runner) runPart(ctx context.Context, name string, arg interface{}) (goja.Value, error) {
	vu, err := r.newVU()
	if err != nil {
		return goja.Undefined(), err
	}
	exp := vu.Runtime.Get("exports").ToObject(vu.Runtime)
	if exp == nil {
		return goja.Undefined(), nil
	}
	fn, ok := goja.AssertFunction(exp.Get(name))
	if !ok {
		return goja.Undefined(), nil
	}

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-ctx.Done()
		vu.Runtime.Interrupt(errInterrupt)
	}()
	v, _, err := vu.runFn(ctx, fn, vu.Runtime.ToValue(arg))
	cancel()
	return v, err
}

type VU struct {
	BundleInstance

	Runner        *Runner
	HTTPTransport *netext.HTTPTransport
	Dialer        *netext.Dialer
	TLSConfig     *tls.Config
	ID            int64
	Iteration     int64

	Console *Console
	BPool   *bpool.BufferPool

	setupData goja.Value

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

func (u *VU) Reconfigure(id int64) error {
	u.ID = id
	u.Iteration = 0
	u.Runtime.Set("__VU", u.ID)
	return nil
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

	// Lazily JS-ify setupData on first run. This is lightweight enough that we can get away with
	// it, and alleviates a problem where setupData wouldn't get populated properly if NewVU() was
	// called before Setup(), which is hard to avoid with how the Executor works w/o complicating
	// the local executor further by deferring SetVUsMax() calls to within the Run() function.
	if u.setupData == nil && u.Runner.setupData != nil {
		u.setupData = u.Runtime.ToValue(u.Runner.setupData)
	}

	// Call the default function.
	_, state, err := u.runFn(ctx, u.Default, u.setupData)
	if err != nil {
		return nil, err
	}
	return state.Samples, nil
}

func (u *VU) runFn(ctx context.Context, fn goja.Callable, args ...goja.Value) (goja.Value, *common.State, error) {
	cookieJar, err := cookiejar.New(nil)
	if err != nil {
		return goja.Undefined(), nil, err
	}

	state := &common.State{
		Logger:        u.Runner.Logger,
		Options:       u.Runner.Bundle.Options,
		Group:         u.Runner.defaultGroup,
		HTTPTransport: u.HTTPTransport,
		Dialer:        u.Dialer,
		TLSConfig:     u.TLSConfig,
		CookieJar:     cookieJar,
		RPSLimit:      u.Runner.RPSLimit,
		BPool:         u.BPool,
		Vu:            u.ID,
		Iteration:     u.Iteration,
	}

	newctx := common.WithRuntime(ctx, u.Runtime)
	newctx = common.WithState(newctx, state)
	*u.Context = newctx

	u.Runtime.Set("__ITER", u.Iteration)
	iter := u.Iteration
	u.Iteration++

	startTime := time.Now()
	v, err := fn(goja.Undefined(), args...) // Actually run the JS script
	endTime := time.Now()

	tags := state.Options.RunTags.CloneTags()
	if state.Options.SystemTags["vu"] {
		tags["vu"] = strconv.FormatInt(u.ID, 10)
	}
	if state.Options.SystemTags["iter"] {
		tags["iter"] = strconv.FormatInt(iter, 10)
	}
	sampleTags := stats.IntoSampleTags(&tags)

	if u.Runner.Bundle.Options.NoConnectionReuse.Bool {
		u.HTTPTransport.CloseIdleConnections()
	}

	bytesWritten := atomic.SwapInt64(&u.Dialer.BytesWritten, 0)
	bytesRead := atomic.SwapInt64(&u.Dialer.BytesRead, 0)

	state.Samples = append(state.Samples,
		stats.Sample{
			Time:   endTime,
			Metric: metrics.DataSent,
			Value:  float64(bytesWritten),
			Tags:   sampleTags},
		stats.Sample{
			Time:   endTime,
			Metric: metrics.DataReceived,
			Value:  float64(bytesRead),
			Tags:   sampleTags},
		stats.Sample{
			Time:   endTime,
			Metric: metrics.IterationDuration,
			Value:  stats.D(endTime.Sub(startTime)),
			Tags:   sampleTags},
	)

	return v, state, err
}
