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
	"time"

	"github.com/dop251/goja"
	"github.com/oxtoacart/bpool"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/viki-org/dnscache"
	"golang.org/x/net/http2"
	"golang.org/x/time/rate"

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/stats"
)

//nolint:gochecknoglobals
var (
	errInterrupt  = errors.New("context cancelled")
	stageSetup    = "setup"
	stageTeardown = "teardown"
)

// Ensure Runner implements the lib.Runner interface
var _ lib.Runner = &Runner{}

type Runner struct {
	Bundle       *Bundle
	Logger       *logrus.Logger
	defaultGroup *lib.Group

	BaseDialer net.Dialer
	Resolver   *dnscache.Resolver
	RPSLimit   *rate.Limiter

	console   *console
	setupData []byte
}

// New returns a new Runner for the provide source
func New(src *loader.SourceData, filesystems map[string]afero.Fs, rtOpts lib.RuntimeOptions) (*Runner, error) {
	bundle, err := NewBundle(src, filesystems, rtOpts)
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
		Logger:       logrus.StandardLogger(),
		defaultGroup: defaultGroup,
		BaseDialer: net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		},
		console:  newConsole(),
		Resolver: dnscache.New(0),
	}

	err = r.SetOptions(r.Bundle.Options)
	return r, err
}

func (r *Runner) MakeArchive() *lib.Archive {
	return r.Bundle.makeArchive()
}

// NewVU returns a initialized VU, which is ready for work.
func (r *Runner) NewVU(_ context.Context, id int64, samples chan<- stats.SampleContainer) (lib.InitializedVU, error) {
	vu, err := r.newVU(id, samples)
	if err != nil {
		return nil, err
	}
	return lib.InitializedVU(vu), nil
}

//nolint: lll, funclen
func (r *Runner) newVU(id int64, samples chan<- stats.SampleContainer) (*VU, error) {
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
		Proxy:               http.ProxyFromEnvironment,
		TLSClientConfig:     tlsConfig,
		DialContext:         dialer.DialContext,
		DisableCompression:  true,
		DisableKeepAlives:   r.Bundle.Options.NoConnectionReuse.Bool,
		MaxIdleConns:        int(r.Bundle.Options.Batch.Int64),
		MaxIdleConnsPerHost: int(r.Bundle.Options.BatchPerHost.Int64),
	}
	_ = http2.ConfigureTransport(transport)

	cookieJar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	vu := &VU{
		ID:             id,
		Iteration:      0,
		BundleInstance: *bi,
		Runner:         r,
		Transport:      transport,
		Dialer:         dialer,
		CookieJar:      cookieJar,
		TLSConfig:      tlsConfig,
		Console:        r.console,
		BPool:          bpool.NewBufferPool(100),
		Samples:        samples,
	}
	vu.Runtime.Set("console", common.Bind(vu.Runtime, vu.Console, vu.Context))
	common.BindToGlobal(vu.Runtime, map[string]interface{}{
		"open": func() {
			common.Throw(vu.Runtime, errors.New("\"open\" function is only available to the init code (aka global scope), see https://docs.k6.io/docs/test-life-cycle for more information"))
		},
	})

	vu.Runtime.Set("__VU", vu.ID)

	return vu, nil
}

func (r *Runner) Setup(ctx context.Context, out chan<- stats.SampleContainer) error {
	setupCtx, setupCancel := context.WithTimeout(
		ctx,
		time.Duration(r.Bundle.Options.SetupTimeout.Duration),
	)
	defer setupCancel()

	v, err := r.runPart(setupCtx, out, stageSetup, nil)
	if err != nil {
		return errors.Wrap(err, stageSetup)
	}
	// r.setupData = nil is special it means undefined from this moment forward
	if goja.IsUndefined(v) {
		r.setupData = nil
		return nil
	}

	r.setupData, err = json.Marshal(v.Export())
	if err != nil {
		return errors.Wrap(err, stageSetup)
	}
	var tmp interface{}
	return json.Unmarshal(r.setupData, &tmp)
}

// GetSetupData returns the setup data as json if Setup() was specified and executed, nil otherwise
func (r *Runner) GetSetupData() []byte {
	return r.setupData
}

// SetSetupData saves the externally supplied setup data as json in the runner, so it can be used in VUs
func (r *Runner) SetSetupData(data []byte) {
	r.setupData = data
}

func (r *Runner) Teardown(ctx context.Context, out chan<- stats.SampleContainer) error {
	teardownCtx, teardownCancel := context.WithTimeout(
		ctx,
		time.Duration(r.Bundle.Options.TeardownTimeout.Duration),
	)
	defer teardownCancel()

	var data interface{}
	if r.setupData != nil {
		if err := json.Unmarshal(r.setupData, &data); err != nil {
			return errors.Wrap(err, stageTeardown)
		}
	} else {
		data = goja.Undefined()
	}

	_, err := r.runPart(teardownCtx, out, stageTeardown, data)
	return err
}

func (r *Runner) GetDefaultGroup() *lib.Group {
	return r.defaultGroup
}

func (r *Runner) GetOptions() lib.Options {
	return r.Bundle.Options
}

func (r *Runner) SetOptions(opts lib.Options) error {
	r.Bundle.Options = opts

	r.RPSLimit = nil
	if rps := opts.RPS; rps.Valid {
		r.RPSLimit = rate.NewLimiter(rate.Limit(rps.Int64), 1)
	}

	//TODO: validate that all exec values are either nil or valid exported methods (or HTTP requests in the future)

	if opts.ConsoleOutput.Valid {
		c, err := newFileConsole(opts.ConsoleOutput.String)
		if err != nil {
			return err
		}

		r.console = c
	}

	return nil
}

// Runs an exported function in its own temporary VU, optionally with an argument. Execution is
// interrupted if the context expires. No error is returned if the part does not exist.
func (r *Runner) runPart(ctx context.Context, out chan<- stats.SampleContainer, name string, arg interface{}) (goja.Value, error) {
	vu, err := r.newVU(0, out)
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
	defer cancel()
	go func() {
		<-ctx.Done()
		vu.Runtime.Interrupt(errInterrupt)
	}()

	group, err := lib.NewGroup(name, r.GetDefaultGroup())
	if err != nil {
		return goja.Undefined(), err
	}

	v, _, _, err := vu.runFn(ctx, group, false, fn, vu.Runtime.ToValue(arg))

	// deadline is reached so we have timeouted but this might've not been registered correctly
	if deadline, ok := ctx.Deadline(); ok && time.Now().After(deadline) {
		// we could have an error that is not errInterrupt in which case we should return it instead
		if err, ok := err.(*goja.InterruptedError); ok && v != nil && err.Value() != errInterrupt {
			//TODO: silence this error?
			return v, err
		}
		// otherwise we have timeouted
		return v, lib.NewTimeoutError(name, r.timeoutErrorDuration(name))
	}
	return v, err
}

// timeoutErrorDuration returns the timeout duration for given stage.
func (r *Runner) timeoutErrorDuration(stage string) time.Duration {
	d := time.Duration(0)
	switch stage {
	case stageSetup:
		return time.Duration(r.Bundle.Options.SetupTimeout.Duration)
	case stageTeardown:
		return time.Duration(r.Bundle.Options.TeardownTimeout.Duration)
	}
	return d
}

type VU struct {
	BundleInstance

	Runner    *Runner
	Transport *http.Transport
	Dialer    *netext.Dialer
	CookieJar *cookiejar.Jar
	TLSConfig *tls.Config
	ID        int64
	Iteration int64

	Console *console
	BPool   *bpool.BufferPool

	Samples chan<- stats.SampleContainer

	setupData goja.Value

	activeCtx context.Context
}

// Verify that VU implements lib.VU
var _ lib.InitializedVU = &VU{}
var _ lib.ActiveVU = &VU{}

// Activate actives an initialized VU, return an active VU, which is ready to do the work.
// It's the caller responsibility to pass context via lib.VUActivationParams.
func (u *VU) Activate(params *lib.VUActivationParams) lib.ActiveVU {
	u.Iteration = 0
	u.activeCtx = params.Ctx

	go func() {
		<-params.Ctx.Done()
		u.Runtime.Interrupt(errInterrupt)

		if params.DeactivateCallback != nil {
			params.DeactivateCallback()
		}
	}()

	return u
}

// RunOnce runs the VU once.
func (u *VU) RunOnce() error {
	// Unmarshall the setupData only the first time for each VU so that VUs are isolated but we
	// still don't use too much CPU in the middle test
	if u.setupData == nil {
		if u.Runner.setupData != nil {
			var data interface{}
			if err := json.Unmarshal(u.Runner.setupData, &data); err != nil {
				return errors.Wrap(err, "RunOnce")
			}
			u.setupData = u.Runtime.ToValue(data)
		} else {
			u.setupData = goja.Undefined()
		}
	}

	// Call the default function.
	_, isFullIteration, totalTime, err := u.runFn(
		u.activeCtx, u.Runner.defaultGroup, true, u.Default, u.setupData,
	)

	// If MinIterationDuration is specified and the iteration wasn't cancelled
	// and was less than it, sleep for the remainder
	if isFullIteration && u.Runner.Bundle.Options.MinIterationDuration.Valid {
		durationDiff := time.Duration(u.Runner.Bundle.Options.MinIterationDuration.Duration) - totalTime
		if durationDiff > 0 {
			time.Sleep(durationDiff)
		}
	}

	return err
}

func (u *VU) runFn(
	ctx context.Context,
	group *lib.Group,
	isDefault bool,
	fn goja.Callable,
	args ...goja.Value,
) (goja.Value, bool, time.Duration, error) {
	cookieJar := u.CookieJar
	if !u.Runner.Bundle.Options.NoCookiesReset.ValueOrZero() {
		var err error
		cookieJar, err = cookiejar.New(nil)
		if err != nil {
			return goja.Undefined(), false, time.Duration(0), err
		}
	}

	state := &lib.State{
		Logger:    u.Runner.Logger,
		Options:   u.Runner.Bundle.Options,
		Group:     group,
		Transport: u.Transport,
		Dialer:    u.Dialer,
		TLSConfig: u.TLSConfig,
		CookieJar: cookieJar,
		RPSLimit:  u.Runner.RPSLimit,
		BPool:     u.BPool,
		Vu:        u.ID,
		Samples:   u.Samples,
		Iteration: u.Iteration,
	}

	newctx := common.WithRuntime(ctx, u.Runtime)
	newctx = lib.WithState(newctx, state)
	*u.Context = newctx

	u.Runtime.Set("__ITER", u.Iteration)
	iter := u.Iteration
	u.Iteration++

	startTime := time.Now()
	v, err := fn(goja.Undefined(), args...) // Actually run the JS script
	endTime := time.Now()

	var isFullIteration bool
	select {
	case <-ctx.Done():
		isFullIteration = false
	default:
		isFullIteration = true
	}

	tags := state.Options.RunTags.CloneTags()
	if state.Options.SystemTags.Has(stats.TagVU) {
		tags["vu"] = strconv.FormatInt(u.ID, 10)
	}
	if state.Options.SystemTags.Has(stats.TagIter) {
		tags["iter"] = strconv.FormatInt(iter, 10)
	}
	if state.Options.SystemTags.Has(stats.TagGroup) {
		tags["group"] = group.Path
	}

	if u.Runner.Bundle.Options.NoVUConnectionReuse.Bool {
		u.Transport.CloseIdleConnections()
	}

	state.Samples <- u.Dialer.GetTrail(startTime, endTime, isFullIteration, isDefault, stats.IntoSampleTags(&tags))

	return v, isFullIteration, endTime.Sub(startTime), err
}
