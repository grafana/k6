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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/dop251/goja"
	"github.com/oxtoacart/bpool"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"golang.org/x/net/http2"
	"golang.org/x/time/rate"

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/consts"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/stats"
)

// Ensure Runner implements the lib.Runner interface
var _ lib.Runner = &Runner{}

type Runner struct {
	Bundle       *Bundle
	Logger       *logrus.Logger
	defaultGroup *lib.Group

	BaseDialer net.Dialer
	Resolver   netext.Resolver
	// TODO: Remove ActualResolver, it's a hack to simplify mocking in tests.
	ActualResolver netext.MultiResolver
	RPSLimit       *rate.Limiter

	console   *console
	setupData []byte
}

// New returns a new Runner for the provide source
func New(
	logger *logrus.Logger, src *loader.SourceData, filesystems map[string]afero.Fs, rtOpts lib.RuntimeOptions,
) (*Runner, error) {
	bundle, err := NewBundle(logger, src, filesystems, rtOpts)
	if err != nil {
		return nil, err
	}

	return newFromBundle(logger, bundle)
}

// NewFromArchive returns a new Runner from the source in the provided archive
func NewFromArchive(logger *logrus.Logger, arc *lib.Archive, rtOpts lib.RuntimeOptions) (*Runner, error) {
	bundle, err := NewBundleFromArchive(logger, arc, rtOpts)
	if err != nil {
		return nil, err
	}

	return newFromBundle(logger, bundle)
}

func newFromBundle(logger *logrus.Logger, b *Bundle) (*Runner, error) {
	defaultGroup, err := lib.NewGroup("", nil)
	if err != nil {
		return nil, err
	}

	defDNS := types.DefaultDNSConfig()
	r := &Runner{
		Bundle:       b,
		Logger:       logger,
		defaultGroup: defaultGroup,
		BaseDialer: net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		},
		console: newConsole(logger),
		Resolver: netext.NewResolver(
			net.LookupIP, 0, defDNS.Select.DNSSelect, defDNS.Policy.DNSPolicy),
		ActualResolver: net.LookupIP,
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
	bi, err := r.Bundle.Instantiate(r.Logger, id)
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
		Dialer:           r.BaseDialer,
		Resolver:         r.Resolver,
		Blacklist:        r.Bundle.Options.BlacklistIPs,
		BlockedHostnames: r.Bundle.Options.BlockedHostnames.Trie,
		Hosts:            r.Bundle.Options.Hosts,
	}
	if r.Bundle.Options.LocalIPs.Valid {
		var ipIndex uint64
		if id > 0 {
			ipIndex = uint64(id - 1)
		}
		dialer.Dialer.LocalAddr = &net.TCPAddr{IP: r.Bundle.Options.LocalIPs.Pool.GetIP(ipIndex)}
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

	vu.state = &lib.State{
		Logger:    vu.Runner.Logger,
		Options:   vu.Runner.Bundle.Options,
		Transport: vu.Transport,
		Dialer:    vu.Dialer,
		TLSConfig: vu.TLSConfig,
		CookieJar: cookieJar,
		RPSLimit:  vu.Runner.RPSLimit,
		BPool:     vu.BPool,
		Vu:        vu.ID,
		Samples:   vu.Samples,
		Iteration: vu.Iteration,
		Tags:      vu.Runner.Bundle.Options.RunTags.CloneTags(),
		Group:     r.defaultGroup,
	}
	vu.Runtime.Set("console", common.Bind(vu.Runtime, vu.Console, vu.Context))

	// This is here mostly so if someone tries they get a nice message
	// instead of "Value is not an object: undefined  ..."
	common.BindToGlobal(vu.Runtime, map[string]interface{}{
		"open": func() {
			common.Throw(vu.Runtime, errors.New(openCantBeUsedOutsideInitContextMsg))
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
		c, err := newFileConsole(opts.ConsoleOutput.String, r.Logger.Formatter)
		if err != nil {
			return err
		}

		r.console = c
	}

	// FIXME: Resolver probably shouldn't be reset here...
	// It's done because the js.Runner is created before the full
	// configuration has been processed, at which point we don't have
	// access to the DNSConfig, and need to wait for this SetOptions
	// call that happens after all config has been assembled.
	// We could make DNSConfig part of RuntimeOptions, but that seems
	// conceptually wrong since the JS runtime doesn't care about it
	// (it needs the actual resolver, not the config), and it would
	// require an additional field on Bundle to pass the config through,
	// which is arguably worse than this.
	if err := r.setResolver(opts.DNS); err != nil {
		return err
	}

	return nil
}

func (r *Runner) setResolver(dns types.DNSConfig) error {
	ttl, err := parseTTL(dns.TTL.String)
	if err != nil {
		return err
	}

	dnsSel := dns.Select
	if !dnsSel.Valid {
		dnsSel = types.DefaultDNSConfig().Select
	}
	dnsPol := dns.Policy
	if !dnsPol.Valid {
		dnsPol = types.DefaultDNSConfig().Policy
	}
	r.Resolver = netext.NewResolver(
		r.ActualResolver, ttl, dnsSel.DNSSelect, dnsPol.DNSPolicy)

	return nil
}

func parseTTL(ttlS string) (time.Duration, error) {
	ttl := time.Duration(0)
	switch ttlS {
	case "inf":
		// cache "infinitely"
		ttl = time.Hour * 24 * 365
	case "0":
		// disable cache
	case "":
		ttlS = types.DefaultDNSConfig().TTL.String
		fallthrough
	default:
		var err error
		ttl, err = types.ParseExtendedDuration(ttlS)
		if ttl < 0 || err != nil {
			return ttl, fmt.Errorf("invalid DNS TTL: %s", ttlS)
		}
	}
	return ttl, nil
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

	ctx = common.WithRuntime(ctx, vu.Runtime)
	ctx = lib.WithState(ctx, vu.state)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		vu.Runtime.Interrupt(context.Canceled)
	}()
	*vu.Context = ctx

	group, err := r.GetDefaultGroup().Group(name)
	if err != nil {
		return goja.Undefined(), err
	}

	if r.Bundle.Options.SystemTags.Has(stats.TagGroup) {
		vu.state.Tags["group"] = group.Path
	}
	vu.state.Group = group

	v, _, _, err := vu.runFn(ctx, false, fn, vu.Runtime.ToValue(arg))

	// deadline is reached so we have timeouted but this might've not been registered correctly
	if deadline, ok := ctx.Deadline(); ok && time.Now().After(deadline) {
		// we could have an error that is not context.Canceled in which case we should return it instead
		if err, ok := err.(*goja.InterruptedError); ok && v != nil && err.Value() != context.Canceled {
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

	state *lib.State
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

// GetID returns the unique VU ID.
func (u *VU) GetID() int64 {
	return u.ID
}

// Activate the VU so it will be able to run code.
func (u *VU) Activate(params *lib.VUActivationParams) lib.ActiveVU {
	u.Runtime.ClearInterrupt()

	if params.Exec == "" {
		params.Exec = consts.DefaultFn
	}

	// Override the preset global env with any custom env vars
	env := make(map[string]string, len(u.env)+len(params.Env))
	for key, value := range u.env {
		env[key] = value
	}
	for key, value := range params.Env {
		env[key] = value
	}
	u.Runtime.Set("__ENV", env)

	opts := u.Runner.Bundle.Options
	// TODO: maybe we can cache the original tags only clone them and add (if any) new tags on top ?
	u.state.Tags = opts.RunTags.CloneTags()
	for k, v := range params.Tags {
		u.state.Tags[k] = v
	}
	if opts.SystemTags.Has(stats.TagVU) {
		u.state.Tags["vu"] = strconv.FormatInt(u.ID, 10)
	}
	if opts.SystemTags.Has(stats.TagIter) {
		u.state.Tags["iter"] = strconv.FormatInt(u.Iteration, 10)
	}
	if opts.SystemTags.Has(stats.TagGroup) {
		u.state.Tags["group"] = u.state.Group.Path
	}
	if opts.SystemTags.Has(stats.TagScenario) {
		u.state.Tags["scenario"] = params.Scenario
	}

	params.RunContext = common.WithRuntime(params.RunContext, u.Runtime)
	params.RunContext = lib.WithState(params.RunContext, u.state)
	*u.Context = params.RunContext

	avu := &ActiveVU{
		VU:                 u,
		VUActivationParams: params,
		busy:               make(chan struct{}, 1),
	}

	go func() {
		// Wait for the run context to be over
		<-params.RunContext.Done()
		// Interrupt the JS runtime
		u.Runtime.Interrupt(context.Canceled)
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
		// Shouldn't happen; this is validated in cmd.validateScenarioConfig()
		panic(fmt.Sprintf("function '%s' not found in exports", u.Exec))
	}

	// Call the exported function.
	_, isFullIteration, totalTime, err := u.runFn(u.RunContext, true, fn, u.setupData)

	// If MinIterationDuration is specified and the iteration wasn't canceled
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
	ctx context.Context, isDefault bool, fn goja.Callable, args ...goja.Value,
) (v goja.Value, isFullIteration bool, t time.Duration, err error) {
	if !u.Runner.Bundle.Options.NoCookiesReset.ValueOrZero() {
		u.state.CookieJar, err = cookiejar.New(nil)
		if err != nil {
			return goja.Undefined(), false, time.Duration(0), err
		}
	}

	opts := &u.Runner.Bundle.Options
	if opts.SystemTags.Has(stats.TagIter) {
		u.state.Tags["iter"] = strconv.FormatInt(u.Iteration, 10)
	}

	// TODO: this seems like the wrong place for the iteration incrementation
	// also this means that teardown and setup have __ITER defined
	// maybe move it to RunOnce ?
	u.Runtime.Set("__ITER", u.Iteration)
	u.Iteration++

	defer func() {
		if r := recover(); r != nil {
			gojaStack := u.Runtime.CaptureCallStack(20, nil)
			err = fmt.Errorf("a panic occurred in VU code but was caught: %s", r)
			// TODO figure out how to use PanicLevel without panicing .. this might require changing
			// the logger we use see
			// https://github.com/sirupsen/logrus/issues/1028
			// https://github.com/sirupsen/logrus/issues/993
			b := new(bytes.Buffer)
			for _, s := range gojaStack {
				s.Write(b)
			}
			u.state.Logger.Log(logrus.ErrorLevel, "panic: ", r, "\n", string(debug.Stack()), "\nGoja stack:\n", b.String())
		}
	}()

	startTime := time.Now()
	v, err = fn(goja.Undefined(), args...) // Actually run the JS script
	endTime := time.Now()

	select {
	case <-ctx.Done():
		isFullIteration = false
	default:
		isFullIteration = true
	}

	if u.Runner.Bundle.Options.NoVUConnectionReuse.Bool {
		u.Transport.CloseIdleConnections()
	}

	u.state.Samples <- u.Dialer.GetTrail(startTime, endTime, isFullIteration, isDefault, stats.NewSampleTags(u.state.Tags))

	return v, isFullIteration, endTime.Sub(startTime), err
}
