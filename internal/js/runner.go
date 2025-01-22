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
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grafana/sobek"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/http2"
	"golang.org/x/time/rate"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/internal/event"
	"go.k6.io/k6/internal/js/eventloop"
	"go.k6.io/k6/internal/loader"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/netext"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
)

// Ensure Runner implements the lib.Runner interface
var _ lib.Runner = &Runner{}

// TODO: https://github.com/grafana/k6/issues/2186
// An advanced TLS support should cover the rid of the warning
//
//nolint:gochecknoglobals
var nameToCertWarning sync.Once

// Runner implements [lib.Runner] and is used to run js tests
type Runner struct {
	Bundle       *Bundle
	preInitState *lib.TestPreInitState

	BaseDialer net.Dialer
	Resolver   netext.Resolver
	// TODO: Remove ActualResolver, it's a hack to simplify mocking in tests.
	ActualResolver netext.MultiResolver
	RPSLimit       *rate.Limiter
	RunTags        *metrics.TagSet

	console    *console
	setupData  []byte
	BufferPool *lib.BufferPool
}

// New returns a new Runner for the provided source
func New(piState *lib.TestPreInitState, src *loader.SourceData, filesystems map[string]fsext.Fs) (*Runner, error) {
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
	defDNS := types.DefaultDNSConfig()
	r := &Runner{
		Bundle:       b,
		preInitState: piState,
		BaseDialer: net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		},
		console: newConsole(piState.Logger),
		Resolver: netext.NewResolver(
			net.LookupIP, 0, defDNS.Select.DNSSelect, defDNS.Policy.DNSPolicy),
		ActualResolver: net.LookupIP,
		BufferPool:     lib.NewBufferPool(),
	}

	err := r.SetOptions(r.Bundle.Options)

	return r, err
}

// MakeArchive creates an Archive of the runner. There should be a corresponding NewFromArchive() function
// that will restore the runner from the archive.
func (r *Runner) MakeArchive() *lib.Archive {
	return r.Bundle.makeArchive()
}

// NewVU returns a new initialized VU.
func (r *Runner) NewVU(
	ctx context.Context, idLocal, idGlobal uint64, samplesOut chan<- metrics.SampleContainer,
) (lib.InitializedVU, error) {
	vu, err := r.newVU(ctx, idLocal, idGlobal, samplesOut)
	if err != nil {
		return nil, err
	}
	return lib.InitializedVU(vu), nil
}

//nolint:funlen
func (r *Runner) newVU(
	ctx context.Context, idLocal, idGlobal uint64, samplesOut chan<- metrics.SampleContainer,
) (*VU, error) {
	// Instantiate a new bundle, make a VU out of it.
	bi, err := r.Bundle.Instantiate(ctx, idLocal)
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
		Hosts:            r.Bundle.Options.Hosts.Trie,
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
		MinVersion:         uint16(tlsVersions.Min), //nolint:gosec
		MaxVersion:         uint16(tlsVersions.Max), //nolint:gosec
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
		tlsConfig.NameToCertificate = nameToCert //nolint:staticcheck
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

	if r.forceHTTP1() {
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
		BufferPool:     r.BufferPool,
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
		BufferPool:     vu.BufferPool,
		VUID:           vu.ID,
		VUIDGlobal:     vu.IDGlobal,
		Samples:        vu.Samples,
		Tags:           lib.NewVUStateTags(vu.Runner.RunTags),
		BuiltinMetrics: r.preInitState.BuiltinMetrics,
		TracerProvider: r.preInitState.TracerProvider,
		Usage:          r.preInitState.Usage,
	}
	vu.moduleVUImpl.state = vu.state
	_ = vu.Runtime.Set("console", vu.Console)

	return vu, nil
}

// forceHTTP1 checks if force http1 env variable has been set in order to force requests to be sent over h1
// TODO: This feature is temporary until #936 is resolved
func (r *Runner) forceHTTP1() bool {
	if r.preInitState.LookupEnv == nil {
		return false
	}
	godebug, _ := r.preInitState.LookupEnv("GODEBUG")
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
	if !r.IsExecutable(consts.SetupFn) {
		// do not init a new transient VU or execute setup() if it wasn't
		// actually defined and exported in the script
		r.preInitState.Logger.Debugf("%s() is not defined or not exported, skipping!", consts.SetupFn)
		return nil
	}
	r.preInitState.Logger.Debugf("Running %s()...", consts.SetupFn)

	setupCtx, setupCancel := context.WithTimeout(ctx, r.getTimeoutFor(consts.SetupFn))
	defer setupCancel()

	v, err := r.runPart(setupCtx, out, consts.SetupFn, nil)
	if err != nil {
		return err
	}
	// r.setupData = nil is special it means undefined from this moment forward
	if sobek.IsUndefined(v) {
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
	if !r.IsExecutable(consts.TeardownFn) {
		// do not init a new transient VU or execute teardown() if it wasn't
		// actually defined and exported in the script
		r.preInitState.Logger.Debugf("%s() is not defined or not exported, skipping!", consts.TeardownFn)
		return nil
	}
	r.preInitState.Logger.Debugf("Running %s()...", consts.TeardownFn)

	teardownCtx, teardownCancel := context.WithTimeout(ctx, r.getTimeoutFor(consts.TeardownFn))
	defer teardownCancel()

	var data interface{}
	if r.setupData != nil {
		if err := json.Unmarshal(r.setupData, &data); err != nil {
			return fmt.Errorf("error unmarshaling setup data for teardown() from JSON: %w", err)
		}
	} else {
		data = sobek.Undefined()
	}
	_, err := r.runPart(teardownCtx, out, consts.TeardownFn, data)
	return err
}

// GetOptions returns the currently calculated [lib.Options] for the given Runner.
func (r *Runner) GetOptions() lib.Options {
	return r.Bundle.Options
}

// IsExecutable returns whether the given name is an exported and
// executable function in the script.
func (r *Runner) IsExecutable(name string) bool {
	_, exists := r.Bundle.callableExports[name]
	return exists
}

// HandleSummary calls the specified summary callback, if supplied.
func (r *Runner) HandleSummary(ctx context.Context, summary *lib.Summary) (map[string]io.Reader, error) {
	summaryDataForJS := summarizeMetricsToObject(summary, r.Bundle.Options, r.setupData)

	out := make(chan metrics.SampleContainer, 100)
	defer close(out)

	go func() { // discard all metrics
		for range out { //nolint:revive
		}
	}()

	summaryCtx, cancel := context.WithTimeout(ctx, r.getTimeoutFor(consts.HandleSummaryFn))
	defer cancel()

	vu, err := r.newVU(summaryCtx, 0, 0, out)
	if err != nil {
		return nil, err
	}

	_ = context.AfterFunc(summaryCtx, func() {
		vu.Runtime.Interrupt(context.Canceled)
	})
	vu.moduleVUImpl.ctx = summaryCtx

	callbackResult := sobek.Undefined()
	fn := vu.getExported(consts.HandleSummaryFn)
	if fn != nil {
		handleSummaryFn, ok := sobek.AssertFunction(fn)
		if !ok {
			return nil, fmt.Errorf("exported identifier %s must be a function", consts.HandleSummaryFn)
		}

		callbackResult, _, _, err = vu.runFn(summaryCtx, false, handleSummaryFn, nil, vu.Runtime.ToValue(summaryDataForJS))
		if err != nil {
			errText, fields := errext.Format(err)
			r.preInitState.Logger.WithFields(fields).Error(errText)
		}
	}

	wrapper := strings.Replace(summaryWrapperLambdaCode, "/*JSLIB_SUMMARY_CODE*/", jslibSummaryCode, 1)
	handleSummaryWrapperRaw, err := vu.Runtime.RunString(wrapper)
	if err != nil {
		return nil, fmt.Errorf("unexpected error while getting the summary wrapper: %w", err)
	}
	handleSummaryWrapper, ok := sobek.AssertFunction(handleSummaryWrapperRaw)
	if !ok {
		return nil, fmt.Errorf("unexpected error did not get a callable summary wrapper")
	}

	wrapperArgs := []sobek.Value{
		callbackResult,
		vu.Runtime.ToValue(r.Bundle.preInitState.RuntimeOptions.SummaryExport.String),
		vu.Runtime.ToValue(summaryDataForJS),
	}
	rawResult, _, _, err := vu.runFn(summaryCtx, false, handleSummaryWrapper, nil, wrapperArgs...)

	if deadlineError := r.checkDeadline(summaryCtx, consts.HandleSummaryFn, rawResult, err); deadlineError != nil {
		return nil, deadlineError
	}

	if err != nil {
		return nil, fmt.Errorf("unexpected error while generating the summary: %w", err)
	}
	return getSummaryResult(rawResult)
}

func (r *Runner) checkDeadline(ctx context.Context, name string, result sobek.Value, err error) error {
	if deadline, ok := ctx.Deadline(); !(ok && time.Now().After(deadline)) {
		return nil
	}

	// deadline is reached so we have timeouted but this might've not been registered correctly
	// we could have an error that is not context.Canceled in which case we should return it instead
	//nolint:errorlint
	if err, ok := err.(*sobek.InterruptedError); ok && result != nil && err.Value() != context.Canceled {
		// TODO: silence this error?
		return err
	}
	// otherwise we have timeouted
	return newTimeoutError(name, r.getTimeoutFor(name))
}

// SetOptions sets the test Options to the provided data and makes necessary changes to the Runner.
func (r *Runner) SetOptions(opts lib.Options) error {
	r.Bundle.Options = opts
	r.RPSLimit = nil
	if rps := opts.RPS; rps.Valid && rps.Int64 > 0 {
		r.RPSLimit = rate.NewLimiter(rate.Limit(rps.Int64), 1)
	}

	// TODO: validate that all exec values are either nil or valid exported methods (or HTTP requests in the future)

	if opts.ConsoleOutput.Valid {
		// TODO: fix logger hack, see https://github.com/grafana/k6/issues/2958
		// and https://github.com/grafana/k6/issues/2968
		var formatter logrus.Formatter = &logrus.JSONFormatter{}
		level := logrus.InfoLevel
		if l, ok := r.preInitState.Logger.(*logrus.Logger); ok { //nolint: forbidigo
			formatter = l.Formatter
			level = l.Level
		}
		c, err := newFileConsole(opts.ConsoleOutput.String, formatter, level)
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

	// FIXME: add tests
	r.RunTags = r.preInitState.Registry.RootTagSet().WithTagsFromMap(r.Bundle.Options.RunTags)

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
	parentCtx context.Context,
	out chan<- metrics.SampleContainer,
	name string,
	arg interface{},
) (sobek.Value, error) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	vu, err := r.newVU(ctx, 0, 0, out)
	if err != nil {
		return sobek.Undefined(), err
	}
	fn, ok := sobek.AssertFunction(vu.getExported(name))
	if !ok {
		return sobek.Undefined(), nil
	}

	_ = context.AfterFunc(ctx, func() {
		vu.Runtime.Interrupt(context.Canceled)
	})
	vu.moduleVUImpl.ctx = ctx

	groupPath, err := lib.NewGroupPath(lib.RootGroupPath, name)
	if err != nil {
		return sobek.Undefined(), err
	}

	if r.Bundle.Options.SystemTags.Has(metrics.TagGroup) {
		vu.state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
			tagsAndMeta.SetSystemTagOrMeta(metrics.TagGroup, groupPath)
		})
	}
	v, _, _, err := vu.runFn(ctx, false, fn, nil, vu.Runtime.ToValue(arg))

	if deadlineError := r.checkDeadline(ctx, name, v, err); deadlineError != nil {
		return nil, deadlineError
	}

	return v, err
}

//nolint:gochecknoglobals
var sobekPromiseType = reflect.TypeOf((*sobek.Promise)(nil))

// unPromisify gets the result of v if it is a promise, otherwise returns v
func unPromisify(v sobek.Value) sobek.Value {
	if !common.IsNullish(v) {
		if v.ExportType() == sobekPromiseType {
			p, ok := v.Export().(*sobek.Promise)
			if !ok {
				panic("Something that was promise did not export to a promise; this shouldn't happen")
			}
			return p.Result()
		}
	}

	return v
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

// VU implements the [lib.VU] interface for the js [Runner].
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

	Console    *console
	BufferPool *lib.BufferPool

	Samples chan<- metrics.SampleContainer

	setupData sobek.Value

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
	//nolint:errcheck,gosec // see https://github.com/grafana/k6/issues/1722#issuecomment-1761173634
	u.Runtime.Set("__ENV", env)

	opts := u.Runner.Bundle.Options

	u.state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
		// Deliberately overwrite tags from previous activations, i.e. ones that
		// might have come from previous scenarios. We also intentionally clear
		// out the metadata, it cannot survive between scenarios either.
		tagsAndMeta.Tags = u.Runner.RunTags.WithTagsFromMap(params.Tags)
		tagsAndMeta.Metadata = nil

		if opts.SystemTags.Has(metrics.TagVU) {
			tagsAndMeta.SetSystemTagOrMeta(metrics.TagVU, strconv.FormatUint(u.ID, 10))
		}
		if opts.SystemTags.Has(metrics.TagIter) {
			tagsAndMeta.SetSystemTagOrMeta(metrics.TagIter, strconv.FormatInt(u.iteration, 10))
		}
		tagsAndMeta.SetSystemTagOrMetaIfEnabled(opts.SystemTags, metrics.TagGroup, lib.RootGroupPath)
		tagsAndMeta.SetSystemTagOrMetaIfEnabled(opts.SystemTags, metrics.TagScenario, params.Scenario)
	})

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

	// Wait for the run context to be over
	context.AfterFunc(ctx, func() {
		// Interrupt the JS runtime
		u.Runtime.Interrupt(context.Canceled)
		// Wait for the VU to stop running, if it was, and prevent it from
		// running again for this activation
		avu.busy <- struct{}{}

		if params.DeactivateCallback != nil {
			params.DeactivateCallback(u)
		}
	})

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
			u.setupData = sobek.Undefined()
		}
	}

	fn := u.getCallableExport(u.Exec)
	if fn == nil {
		// Shouldn't happen; this is validated in cmd.validateScenarioConfig()
		panic(fmt.Sprintf("function '%s' not found in exports", u.Exec))
	}

	u.incrIteration()
	if err := u.Runtime.Set("__ITER", u.iteration); err != nil {
		panic(fmt.Errorf("error setting __ITER in Sobek runtime: %w", err))
	}

	ctx, cancel := context.WithCancel(u.RunContext)
	defer cancel()
	u.moduleVUImpl.ctx = ctx

	eventIterData := event.IterData{
		Iteration:    u.iteration,
		VUID:         u.ID,
		ScenarioName: u.scenarioName,
	}

	u.emitAndWaitEvent(&event.Event{Type: event.IterStart, Data: eventIterData})

	// Call the exported function.
	_, isFullIteration, totalTime, err := u.runFn(ctx, true, fn, cancel, u.setupData)
	if err != nil {
		var x *sobek.InterruptedError
		if errors.As(err, &x) {
			if v, ok := x.Value().(*errext.InterruptError); ok {
				v.Reason = x.Error()
				err = v
			}
		}
		eventIterData.Error = err
	}

	u.emitAndWaitEvent(&event.Event{Type: event.IterEnd, Data: eventIterData})

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

func (u *ActiveVU) emitAndWaitEvent(evt *event.Event) {
	waitDone := u.moduleVUImpl.events.local.Emit(evt)
	waitCtx, waitCancel := context.WithTimeout(u.RunContext, 30*time.Minute)
	defer waitCancel()
	if werr := waitDone(waitCtx); werr != nil {
		u.state.Logger.WithError(werr).Warn()
	}
}

func (u *VU) getExported(name string) sobek.Value {
	return u.BundleInstance.getExported(name)
}

// if isDefault is true, cancel also needs to be provided and it should cancel the provided context
// TODO remove the need for the above through refactoring of this function and its callees
func (u *VU) runFn(
	ctx context.Context, isDefault bool, fn sobek.Callable, cancel func(), args ...sobek.Value,
) (v sobek.Value, isFullIteration bool, t time.Duration, err error) {
	if !u.Runner.Bundle.Options.NoCookiesReset.ValueOrZero() {
		u.state.CookieJar, err = cookiejar.New(nil)
		if err != nil {
			return sobek.Undefined(), false, time.Duration(0), err
		}
	}

	opts := &u.Runner.Bundle.Options

	if opts.SystemTags.Has(metrics.TagIter) {
		u.state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
			tagsAndMeta.SetSystemTagOrMeta(metrics.TagIter, strconv.FormatInt(u.state.Iteration, 10))
		})
	}

	startTime := time.Now()

	if u.moduleVUImpl.eventLoop == nil {
		u.moduleVUImpl.eventLoop = eventloop.New(u.moduleVUImpl)
	}
	err = u.moduleVUImpl.eventLoop.Start(func() (err error) {
		v, err = fn(sobek.Undefined(), args...) // Actually run the JS script
		return err
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
	var exception *sobek.Exception
	if errors.As(err, &exception) {
		err = &scriptExceptionError{inner: exception}
	}

	if u.Runner.Bundle.Options.NoVUConnectionReuse.Bool {
		u.Transport.CloseIdleConnections()
	}

	builtinMetrics := u.Runner.preInitState.BuiltinMetrics
	ctm := u.state.Tags.GetCurrentValues()
	u.state.Samples <- u.Dialer.IOSamples(endTime, ctm, builtinMetrics)

	if isFullIteration && isDefault {
		u.state.Samples <- iterationSamples(startTime, endTime, ctm, builtinMetrics)
	}

	v = unPromisify(v)

	return v, isFullIteration, endTime.Sub(startTime), err
}

func iterationSamples(
	startTime, endTime time.Time, ctm metrics.TagsAndMeta, builtinMetrics *metrics.BuiltinMetrics,
) metrics.Samples {
	return metrics.Samples([]metrics.Sample{
		{
			TimeSeries: metrics.TimeSeries{
				Metric: builtinMetrics.IterationDuration,
				Tags:   ctm.Tags,
			},
			Time:     endTime,
			Metadata: ctm.Metadata,
			Value:    metrics.D(endTime.Sub(startTime)),
		},
		{
			TimeSeries: metrics.TimeSeries{
				Metric: builtinMetrics.Iterations,
				Tags:   ctm.Tags,
			},
			Time:     endTime,
			Metadata: ctm.Metadata,
			Value:    1,
		},
	})
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

type scriptExceptionError struct {
	inner *sobek.Exception
}

var _ interface {
	errext.Exception
	errext.HasExitCode
	errext.HasHint
	errext.HasAbortReason
} = &scriptExceptionError{}

func (s *scriptExceptionError) Error() string {
	// this calls String instead of error so that by default if it's printed to print the stacktrace
	return s.inner.String()
}

func (s *scriptExceptionError) StackTrace() string {
	return s.inner.String()
}

func (s *scriptExceptionError) Unwrap() error {
	return s.inner
}

func (s *scriptExceptionError) Hint() string {
	return "script exception"
}

func (s *scriptExceptionError) AbortReason() errext.AbortReason {
	return errext.AbortedByScriptError
}

func (s *scriptExceptionError) ExitCode() exitcodes.ExitCode {
	return exitcodes.ScriptException
}
