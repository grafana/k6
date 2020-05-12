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
	"fmt"
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
	"github.com/loadimpact/k6/lib/consts"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/stats"
)

//nolint:gochecknoglobals
var errInterrupt = errors.New("context cancelled")

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

// NewVU returns a new initialized VU.
func (r *Runner) NewVU(id int64, samplesOut chan<- stats.SampleContainer) (lib.InitializedVU, error) {
	vu, err := r.newVU(id, samplesOut)
	if err != nil {
		return nil, err
	}
	return lib.InitializedVU(vu), nil
}

// nolint:funlen
func (r *Runner) newVU(id int64, samplesOut chan<- stats.SampleContainer) (*VU, error) {
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
		BundleInstance: *bi,
		Runner:         r,
		Transport:      transport,
		Dialer:         dialer,
		CookieJar:      cookieJar,
		TLSConfig:      tlsConfig,
		Console:        r.console,
		BPool:          bpool.NewBufferPool(100),
		Samples:        samplesOut,
	}
	vu.Runtime.Set("__VU", vu.ID)
	vu.Runtime.Set("console", common.Bind(vu.Runtime, vu.Console, vu.Context))
	common.BindToGlobal(vu.Runtime, map[string]interface{}{
		"open": func() {
			common.Throw(vu.Runtime, errors.New(
				`The "open()" function is only available to init code (aka the global scope), see `+
					` https://k6.io/docs/using-k6/test-life-cycle for more information`,
			))
		},
	})

	return vu, nil
}

func (r *Runner) Setup(ctx context.Context, out chan<- stats.SampleContainer) error {
	setupCtx, setupCancel := context.WithTimeout(
		ctx,
		time.Duration(r.Bundle.Options.SetupTimeout.Duration),
	)
	defer setupCancel()

	v, err := r.runPart(setupCtx, out, consts.SetupFn, nil)
	if err != nil {
		return err
	}
	// r.setupData = nil is special it means undefined from this moment forward
	if goja.IsUndefined(v) {
		r.setupData = nil
		return nil
	}

	r.setupData, err = json.Marshal(v.Export())
	if err != nil {
		return errors.Wrap(err, consts.SetupFn)
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
			return errors.Wrap(err, consts.TeardownFn)
		}
	} else {
		data = goja.Undefined()
	}
	_, err := r.runPart(teardownCtx, out, consts.TeardownFn, data)
	return err
}

func (r *Runner) GetDefaultGroup() *lib.Group {
	return r.defaultGroup
}

func (r *Runner) GetOptions() lib.Options {
	return r.Bundle.Options
}

// IsExecutable returns whether the given name is an exported and
// executable function in the script.
func (r *Runner) IsExecutable(name string) bool {
	_, exists := r.Bundle.exports[name]
	return exists
}

func (r *Runner) SetOptions(opts lib.Options) error {
	r.Bundle.Options = opts

	r.RPSLimit = nil
	if rps := opts.RPS; rps.Valid {
		r.RPSLimit = rate.NewLimiter(rate.Limit(rps.Int64), 1)
	}

	// TODO: validate that all exec values are either nil or valid exported methods (or HTTP requests in the future)

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

	v, _, _, err := vu.runFn(ctx, group, false, nil, fn, vu.Runtime.ToValue(arg))

	// deadline is reached so we have timeouted but this might've not been registered correctly
	if deadline, ok := ctx.Deadline(); ok && time.Now().After(deadline) {
		// we could have an error that is not errInterrupt in which case we should return it instead
		if err, ok := err.(*goja.InterruptedError); ok && v != nil && err.Value() != errInterrupt {
			// TODO: silence this error?
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
	case consts.SetupFn:
		return time.Duration(r.Bundle.Options.SetupTimeout.Duration)
	case consts.TeardownFn:
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
}

// Verify that interfaces are implemented
var (
	_ lib.ActiveVU      = &ActiveVU{}
	_ lib.InitializedVU = &VU{}
)

// ActiveVU holds a VU and its activation parameters
type ActiveVU struct {
	*VU
	*lib.VUActivationParams
	busy chan struct{}
}

// Activate the VU so it will be able to run code.
func (u *VU) Activate(params *lib.VUActivationParams) lib.ActiveVU {
	u.Runtime.ClearInterrupt()

	if params.Exec == "" {
		params.Exec = consts.DefaultFn
	}

	// Override the preset global env with any custom env vars
	if len(params.Env) > 0 {
		env := u.Runtime.Get("__ENV").Export().(map[string]string)
		for key, value := range params.Env {
			env[key] = value
		}
		u.Runtime.Set("__ENV", env)
	}

	avu := &ActiveVU{
		VU:                 u,
		VUActivationParams: params,
		busy:               make(chan struct{}, 1),
	}

	go func() {
		// Wait for the run context to be over
		<-params.RunContext.Done()
		// Interrupt the JS runtime
		u.Runtime.Interrupt(errInterrupt)
		// Wait for the VU to stop running, if it was, and prevent it from
		// running again for this activation
		avu.busy <- struct{}{}

		if params.DeactivateCallback != nil {
			params.DeactivateCallback(u)
		}
	}()

	return avu
}

// RunOnce runs the configured Exec function once.
func (u *ActiveVU) RunOnce() error {
	select {
	case <-u.RunContext.Done():
		return u.RunContext.Err() // we are done, return
	case u.busy <- struct{}{}:
		// nothing else can run now, and the VU cannot be deactivated
	}
	defer func() {
		<-u.busy // unlock deactivation again
	}()

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

	fn, ok := u.exports[u.Exec]
	if !ok {
		// Shouldn't happen; this is validated in ExecutionScheduler.Init()
		panic(fmt.Sprintf("function '%s' not found in exports", u.Exec))
	}

	// Call the exported function.
	_, isFullIteration, totalTime, err := u.runFn(
		u.RunContext, u.Runner.defaultGroup, true, u.Tags, fn, u.setupData,
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
	ctx context.Context, group *lib.Group, isDefault bool, customTags map[string]string,
	fn goja.Callable, args ...goja.Value,
) (goja.Value, bool, time.Duration, error) {
	cookieJar := u.CookieJar
	if !u.Runner.Bundle.Options.NoCookiesReset.ValueOrZero() {
		var err error
		cookieJar, err = cookiejar.New(nil)
		if err != nil {
			return goja.Undefined(), false, time.Duration(0), err
		}
	}

	opts := &u.Runner.Bundle.Options
	tags := opts.RunTags.CloneTags()
	for k, v := range customTags {
		tags[k] = v
	}
	if opts.SystemTags.Has(stats.TagVU) {
		tags["vu"] = strconv.FormatInt(u.ID, 10)
	}
	if opts.SystemTags.Has(stats.TagIter) {
		tags["iter"] = strconv.FormatInt(u.Iteration, 10)
	}
	if opts.SystemTags.Has(stats.TagGroup) {
		tags["group"] = group.Path
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
		Tags:      tags,
	}

	newctx := common.WithRuntime(ctx, u.Runtime)
	newctx = lib.WithState(newctx, state)
	*u.Context = newctx

	u.Runtime.Set("__ITER", u.Iteration)
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

	if u.Runner.Bundle.Options.NoVUConnectionReuse.Bool {
		u.Transport.CloseIdleConnections()
	}

	state.Samples <- u.Dialer.GetTrail(startTime, endTime, isFullIteration, isDefault, stats.IntoSampleTags(&tags))

	return v, isFullIteration, endTime.Sub(startTime), err
}
