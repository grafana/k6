package js

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/oxtoacart/bpool"
	"github.com/spf13/afero"
	"golang.org/x/net/http2"
	"golang.org/x/time/rate"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/eventloop"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/lib/netext"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/loader"
	"go.k6.io/k6/metrics"
)

// Ensure Runner implements the lib.Runner interface
var _ lib.Runner = &Runner{}

// TODO: https://github.com/grafana/k6/issues/2186
// An advanced TLS support should cover the rid of the warning
//
//nolint:gochecknoglobals
var nameToCertWarning sync.Once

type Runner struct {
	Bundle       *Bundle
	preInitState *lib.TestPreInitState
	defaultGroup *lib.Group

	BaseDialer net.Dialer
	Resolver   netext.Resolver
	// TODO: Remove ActualResolver, it's a hack to simplify mocking in tests.
	ActualResolver netext.MultiResolver
	RPSLimit       *rate.Limiter

	console   *console
	setupData []byte
}

// New returns a new Runner for the provided source
func New(piState *lib.TestPreInitState, src *loader.SourceData, filesystems map[string]afero.Fs) (*Runner, error) {
	bundle, err := NewBundle(piState, src, filesystems)
	if err != nil {
		return nil, err
	}

	return NewFromBundle(piState, bundle)
}

// NewFromArchive returns a new Runner from the source in the provided archive
func NewFromArchive(piState *lib.TestPreInitState, arc *lib.Archive) (*Runner, error) {
	bundle, err := NewBundleFromArchive(piState, arc)
	if err != nil {
		return nil, err
	}

	return NewFromBundle(piState, bundle)
}

// NewFromBundle returns a new Runner from the provided Bundle
func NewFromBundle(piState *lib.TestPreInitState, b *Bundle) (*Runner, error) {
	defaultGroup, err := lib.NewGroup("", nil)
	if err != nil {
		return nil, err
	}

	defDNS := types.DefaultDNSConfig()
	r := &Runner{
		Bundle:       b,
		preInitState: piState,
		defaultGroup: defaultGroup,
		BaseDialer: net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		},
		console: newConsole(piState.Logger),
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
func (r *Runner) NewVU(idLocal, idGlobal uint64, samplesOut chan<- metrics.SampleContainer) (lib.InitializedVU, error) {
	vu, err := r.newVU(idLocal, idGlobal, samplesOut)
	if err != nil {
		return nil, err
	}
	return lib.InitializedVU(vu), nil
}

//nolint:funlen
func (r *Runner) newVU(idLocal, idGlobal uint64, samplesOut chan<- metrics.SampleContainer) (*VU, error) {
	// Instantiate a new bundle, make a VU out of it.
	bi, err := r.Bundle.Instantiate(r.preInitState.Logger, idLocal)
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
		cert, errC := auth.Certificate()
		if errC != nil {
			return nil, errC
		}
		certs[i] = *cert
		for _, name := range auth.Domains {
			nameToCert[name] = cert
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
		if idLocal > 0 {
			ipIndex = idLocal - 1
		}
		dialer.Dialer.LocalAddr = &net.TCPAddr{IP: r.Bundle.Options.LocalIPs.Pool.GetIP(ipIndex)}
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: r.Bundle.Options.InsecureSkipTLSVerify.Bool, //nolint:gosec
		CipherSuites:       cipherSuites,
		MinVersion:         uint16(tlsVersions.Min),
		MaxVersion:         uint16(tlsVersions.Max),
		Certificates:       certs,
		Renegotiation:      tls.RenegotiateFreelyAsClient,
		KeyLogWriter:       r.preInitState.KeyLogger,
	}
	// Follow NameToCertificate in https://pkg.go.dev/crypto/tls@go1.17.6#Config, leave this field nil
	// when it is empty
	if len(nameToCert) > 0 {
		nameToCertWarning.Do(func() {
			r.preInitState.Logger.Warn(
				"tlsAuth.domains option could be removed in the next releases, it's recommended to leave it empty " +
					"and let k6 automatically detect from the provided certificate. It follows the Go's NameToCertificate " +
					"deprecation - https://pkg.go.dev/crypto/tls@go1.17#Config.",
			)
		})
		//nolint:staticcheck // ignore SA1019 we can deprecate it but we have to continue to support the previous code.
		tlsConfig.NameToCertificate = nameToCert
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

	if forceHTTP1() {
		transport.TLSNextProto = make(map[string]func(string, *tls.Conn) http.RoundTripper) // send over h1 protocol
	} else {
		_ = http2.ConfigureTransport(transport) // send over h2 protocol
	}

	cookieJar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	vu := &VU{
		ID:             idLocal,
		IDGlobal:       idGlobal,
		iteration:      int64(-1),
		BundleInstance: *bi,
		Runner:         r,
		Transport:      transport,
		Dialer:         dialer,
		CookieJar:      cookieJar,
		TLSConfig:      tlsConfig,
		Console:        r.console,
		BPool:          bpool.NewBufferPool(100),
		Samples:        samplesOut,
		scenarioIter:   make(map[string]uint64),
	}

	vu.state = &lib.State{
		Logger:         vu.Runner.preInitState.Logger,
		Options:        vu.Runner.Bundle.Options,
		Transport:      vu.Transport,
		Dialer:         vu.Dialer,
		TLSConfig:      vu.TLSConfig,
		CookieJar:      cookieJar,
		RPSLimit:       vu.Runner.RPSLimit,
		BPool:          vu.BPool,
		VUID:           vu.ID,
		VUIDGlobal:     vu.IDGlobal,
		Samples:        vu.Samples,
		Tags:           lib.NewTagMap(copyStringMap(vu.Runner.Bundle.Options.RunTags)),
		Group:          r.defaultGroup,
		BuiltinMetrics: r.preInitState.BuiltinMetrics,
	}
	vu.moduleVUImpl.state = vu.state
	_ = vu.Runtime.Set("console", vu.Console)

	// This is here mostly so if someone tries they get a nice message
	// instead of "Value is not an object: undefined  ..."
	_ = vu.Runtime.GlobalObject().Set("open",
		func() {
			common.Throw(vu.Runtime, errors.New(openCantBeUsedOutsideInitContextMsg))
		})

	return vu, nil
}

// forceHTTP1 checks if force http1 env variable has been set in order to force requests to be sent over h1
// TODO: This feature is temporary until #936 is resolved
func forceHTTP1() bool {
	godebug := os.Getenv("GODEBUG")
	if godebug == "" {
		return false
	}
	variables := strings.SplitAfter(godebug, ",")

	for _, v := range variables {
		if strings.Trim(v, ",") == "http2client=0" {
			return true
		}
	}
	return false
}

// Setup runs the setup function if there is one and sets the setupData to the returned value
func (r *Runner) Setup(ctx context.Context, out chan<- metrics.SampleContainer) error {
	setupCtx, setupCancel := context.WithTimeout(ctx, r.getTimeoutFor(consts.SetupFn))
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
		return fmt.Errorf("error marshaling setup() data to JSON: %w", err)
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

// Teardown runs the teardown function if there is one.
func (r *Runner) Teardown(ctx context.Context, out chan<- metrics.SampleContainer) error {
	teardownCtx, teardownCancel := context.WithTimeout(ctx, r.getTimeoutFor(consts.TeardownFn))
	defer teardownCancel()

	var data interface{}
	if r.setupData != nil {
		if err := json.Unmarshal(r.setupData, &data); err != nil {
			return fmt.Errorf("error unmarshaling setup data for teardown() from JSON: %w", err)
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

// HandleSummary calls the specified summary callback, if supplied.
func (r *Runner) HandleSummary(ctx context.Context, summary *lib.Summary) (map[string]io.Reader, error) {
	summaryDataForJS := summarizeMetricsToObject(summary, r.Bundle.Options, r.setupData)

	out := make(chan metrics.SampleContainer, 100)
	defer close(out)

	go func() { // discard all metrics
		for range out {
		}
	}()

	vu, err := r.newVU(0, 0, out)
	if err != nil {
		return nil, err
	}

	handleSummaryFn := goja.Undefined()
	fn := vu.getExported(consts.HandleSummaryFn)
	if _, ok := goja.AssertFunction(fn); ok {
		handleSummaryFn = fn
	} else if fn != nil {
		return nil, fmt.Errorf("exported identifier %s must be a function", consts.HandleSummaryFn)
	}

	ctx, cancel := context.WithTimeout(ctx, r.getTimeoutFor(consts.HandleSummaryFn))
	defer cancel()
	go func() {
		<-ctx.Done()
		vu.Runtime.Interrupt(context.Canceled)
	}()
	vu.moduleVUImpl.ctx = ctx

	wrapper := strings.Replace(summaryWrapperLambdaCode, "/*JSLIB_SUMMARY_CODE*/", jslibSummaryCode, 1)
	handleSummaryWrapperRaw, err := vu.Runtime.RunString(wrapper)
	if err != nil {
		return nil, fmt.Errorf("unexpected error while getting the summary wrapper: %w", err)
	}
	handleSummaryWrapper, ok := goja.AssertFunction(handleSummaryWrapperRaw)
	if !ok {
		return nil, fmt.Errorf("unexpected error did not get a callable summary wrapper")
	}

	wrapperArgs := []goja.Value{
		handleSummaryFn,
		vu.Runtime.ToValue(r.Bundle.RuntimeOptions.SummaryExport.String),
		vu.Runtime.ToValue(summaryDataForJS),
	}
	rawResult, _, _, err := vu.runFn(ctx, false, handleSummaryWrapper, nil, wrapperArgs...)

	// TODO: refactor the whole JS runner to avoid copy-pasting these complicated bits...
	// deadline is reached so we have timeouted but this might've not been registered correctly
	if deadline, ok := ctx.Deadline(); ok && time.Now().After(deadline) {
		// we could have an error that is not context.Canceled in which case we should return it instead
		if err, ok := err.(*goja.InterruptedError); ok && rawResult != nil && err.Value() != context.Canceled {
			// TODO: silence this error?
			return nil, err
		}
		// otherwise we have timeouted
		return nil, newTimeoutError(consts.HandleSummaryFn, r.getTimeoutFor(consts.HandleSummaryFn))
	}

	if err != nil {
		return nil, fmt.Errorf("unexpected error while generating the summary: %w", err)
	}
	return getSummaryResult(rawResult)
}

func (r *Runner) SetOptions(opts lib.Options) error {
	r.Bundle.Options = opts
	r.RPSLimit = nil
	if rps := opts.RPS; rps.Valid && rps.Int64 > 0 {
		r.RPSLimit = rate.NewLimiter(rate.Limit(rps.Int64), 1)
	}

	// TODO: validate that all exec values are either nil or valid exported methods (or HTTP requests in the future)

	if opts.ConsoleOutput.Valid {
		c, err := newFileConsole(opts.ConsoleOutput.String, r.preInitState.Logger.Formatter)
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
func (r *Runner) runPart(
	ctx context.Context,
	out chan<- metrics.SampleContainer,
	name string,
	arg interface{},
) (goja.Value, error) {
	vu, err := r.newVU(0, 0, out)
	if err != nil {
		return goja.Undefined(), err
	}
	fn, ok := goja.AssertFunction(vu.getExported(name))
	if !ok {
		return goja.Undefined(), nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		vu.Runtime.Interrupt(context.Canceled)
	}()
	vu.moduleVUImpl.ctx = ctx

	group, err := r.GetDefaultGroup().Group(name)
	if err != nil {
		return goja.Undefined(), err
	}

	if r.Bundle.Options.SystemTags.Has(metrics.TagGroup) {
		vu.state.Tags.Set("group", group.Path)
	}
	vu.state.Group = group

	v, _, _, err := vu.runFn(ctx, false, fn, nil, vu.Runtime.ToValue(arg))

	// deadline is reached so we have timeouted but this might've not been registered correctly
	if deadline, ok := ctx.Deadline(); ok && time.Now().After(deadline) {
		// we could have an error that is not context.Canceled in which case we should return it instead
		if err, ok := err.(*goja.InterruptedError); ok && v != nil && err.Value() != context.Canceled {
			// TODO: silence this error?
			return v, err
		}
		// otherwise we have timeouted
		return v, newTimeoutError(name, r.getTimeoutFor(name))
	}
	return v, err
}

// getTimeoutFor returns the timeout duration for given special script function.
func (r *Runner) getTimeoutFor(stage string) time.Duration {
	d := time.Duration(0)
	switch stage {
	case consts.SetupFn:
		return r.Bundle.Options.SetupTimeout.TimeDuration()
	case consts.TeardownFn:
		return r.Bundle.Options.TeardownTimeout.TimeDuration()
	case consts.HandleSummaryFn:
		return 2 * time.Minute // TODO: make configurable
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
	ID        uint64 // local to the current instance
	IDGlobal  uint64 // global across all instances
	iteration int64

	Console *console
	BPool   *bpool.BufferPool

	Samples chan<- metrics.SampleContainer

	setupData goja.Value

	state *lib.State
	// count of iterations executed by this VU in each scenario
	scenarioIter map[string]uint64
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

	scenarioName              string
	getNextIterationCounters  func() (uint64, uint64)
	scIterLocal, scIterGlobal uint64
}

// GetID returns the unique VU ID.
func (u *VU) GetID() uint64 {
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
	u.state.Tags = lib.NewTagMap(copyStringMap(opts.RunTags))
	for k, v := range params.Tags {
		u.state.Tags.Set(k, v)
	}
	if opts.SystemTags.Has(metrics.TagVU) {
		u.state.Tags.Set("vu", strconv.FormatUint(u.ID, 10))
	}
	if opts.SystemTags.Has(metrics.TagIter) {
		u.state.Tags.Set("iter", strconv.FormatInt(u.iteration, 10))
	}
	if opts.SystemTags.Has(metrics.TagGroup) {
		u.state.Tags.Set("group", u.state.Group.Path)
	}
	if opts.SystemTags.Has(metrics.TagScenario) {
		u.state.Tags.Set("scenario", params.Scenario)
	}

	ctx := params.RunContext
	u.moduleVUImpl.ctx = ctx

	u.state.GetScenarioVUIter = func() uint64 {
		return u.scenarioIter[params.Scenario]
	}

	avu := &ActiveVU{
		VU:                       u,
		VUActivationParams:       params,
		busy:                     make(chan struct{}, 1),
		scenarioName:             params.Scenario,
		scIterLocal:              ^uint64(0),
		scIterGlobal:             ^uint64(0),
		getNextIterationCounters: params.GetNextIterationCounters,
	}

	u.state.GetScenarioLocalVUIter = func() uint64 {
		return avu.scIterLocal
	}
	u.state.GetScenarioGlobalVUIter = func() uint64 {
		return avu.scIterGlobal
	}

	go func() {
		// Wait for the run context to be over
		<-ctx.Done()
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
				return fmt.Errorf("error unmarshaling setup data for the iteration from JSON: %w", err)
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

	u.incrIteration()
	if err := u.Runtime.Set("__ITER", u.iteration); err != nil {
		panic(fmt.Errorf("error setting __ITER in goja runtime: %w", err))
	}

	ctx, cancel := context.WithCancel(u.RunContext)
	defer cancel()
	u.moduleVUImpl.ctx = ctx
	// Call the exported function.
	_, isFullIteration, totalTime, err := u.runFn(ctx, true, fn, cancel, u.setupData)
	if err != nil {
		var x *goja.InterruptedError
		if errors.As(err, &x) {
			if v, ok := x.Value().(*errext.InterruptError); ok {
				v.Reason = x.Error()
				err = v
			}
		}
	}

	// If MinIterationDuration is specified and the iteration wasn't canceled
	// and was less than it, sleep for the remainder
	if isFullIteration && u.Runner.Bundle.Options.MinIterationDuration.Valid {
		durationDiff := u.Runner.Bundle.Options.MinIterationDuration.TimeDuration() - totalTime
		if durationDiff > 0 {
			select {
			case <-time.After(durationDiff):
			case <-u.RunContext.Done():
			}
		}
	}

	return err
}

func (u *VU) getExported(name string) goja.Value {
	return u.BundleInstance.pgm.module.Get("exports").ToObject(u.Runtime).Get(name)
}

// if isDefault is true, cancel also needs to be provided and it should cancel the provided context
// TODO remove the need for the above through refactoring of this function and its callees
func (u *VU) runFn(
	ctx context.Context, isDefault bool, fn goja.Callable, cancel func(), args ...goja.Value,
) (v goja.Value, isFullIteration bool, t time.Duration, err error) {
	if !u.Runner.Bundle.Options.NoCookiesReset.ValueOrZero() {
		u.state.CookieJar, err = cookiejar.New(nil)
		if err != nil {
			return goja.Undefined(), false, time.Duration(0), err
		}
	}

	opts := &u.Runner.Bundle.Options
	if opts.SystemTags.Has(metrics.TagIter) {
		u.state.Tags.Set("iter", strconv.FormatInt(u.state.Iteration, 10))
	}

	startTime := time.Now()

	if u.moduleVUImpl.eventLoop == nil {
		u.moduleVUImpl.eventLoop = eventloop.New(u.moduleVUImpl)
	}
	err = common.RunWithPanicCatching(u.state.Logger, u.Runtime, func() error {
		return u.moduleVUImpl.eventLoop.Start(func() (err error) {
			v, err = fn(goja.Undefined(), args...) // Actually run the JS script
			return err
		})
	})

	select {
	case <-ctx.Done():
		isFullIteration = false
	default:
		isFullIteration = true
	}

	if cancel != nil {
		cancel()
		u.moduleVUImpl.eventLoop.WaitOnRegistered()
	}
	endTime := time.Now()
	var exception *goja.Exception
	if errors.As(err, &exception) {
		err = &scriptException{inner: exception}
	}

	if u.Runner.Bundle.Options.NoVUConnectionReuse.Bool {
		u.Transport.CloseIdleConnections()
	}

	sampleTags := metrics.NewSampleTags(u.state.CloneTags())
	u.state.Samples <- u.Dialer.GetTrail(
		startTime, endTime, isFullIteration, isDefault, sampleTags, u.Runner.preInitState.BuiltinMetrics)

	return v, isFullIteration, endTime.Sub(startTime), err
}

func (u *ActiveVU) incrIteration() {
	u.iteration++
	u.state.Iteration = u.iteration

	if _, ok := u.scenarioIter[u.scenarioName]; ok {
		u.scenarioIter[u.scenarioName]++
	} else {
		u.scenarioIter[u.scenarioName] = 0
	}
	// TODO remove this
	if u.getNextIterationCounters != nil {
		u.scIterLocal, u.scIterGlobal = u.getNextIterationCounters()
	}
}

type scriptException struct {
	inner *goja.Exception
}

var (
	_ errext.Exception   = &scriptException{}
	_ errext.HasExitCode = &scriptException{}
	_ errext.HasHint     = &scriptException{}
)

func (s *scriptException) Error() string {
	// this calls String instead of error so that by default if it's printed to print the stacktrace
	return s.inner.String()
}

func (s *scriptException) StackTrace() string {
	return s.inner.String()
}

func (s *scriptException) Unwrap() error {
	return s.inner
}

func (s *scriptException) Hint() string {
	return "script exception"
}

func (s *scriptException) ExitCode() exitcodes.ExitCode {
	return exitcodes.ScriptException
}

func copyStringMap(m map[string]string) map[string]string {
	clone := make(map[string]string, len(m))
	for ktag, vtag := range m {
		clone[ktag] = vtag
	}
	return clone
}
