package jsexec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"runtime/pprof"
	rtrace "runtime/trace"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/pprof/profile"
	"github.com/grafana/sobek"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
)

const (
	// MetadataTraceIDKey is the output metadata key for JS trace correlation.
	MetadataTraceIDKey = "js_trace_id"
	// MetadataProfileIDKey is the output metadata key for JS profile correlation.
	MetadataProfileIDKey = "js_profile_id"
)

// Config defines JS observability capture settings.
type Config struct {
	Enabled                   bool
	Scope                     ProfileScope
	CPUProfilePath            string
	RuntimeTracePath          string
	ProfileID                 string
	MaxJSStackLabelLen        int
	FirstRunnerMemMaxBytes    int64
	FirstRunnerMemStepPercent int64
	Logf                      func(format string, args ...any)
}

// ProfileScope controls where JS profiling is captured.
type ProfileScope string

const (
	// ScopeInit captures only init-context runtime execution.
	ScopeInit ProfileScope = "init"
	// ScopeVU captures only VU runtime execution.
	ScopeVU ProfileScope = "vu"
	// ScopeCombined captures both init and VU runtime execution.
	ScopeCombined ProfileScope = "combined"
)

func normalizeScope(s string) ProfileScope {
	switch ProfileScope(strings.ToLower(strings.TrimSpace(s))) {
	case ScopeInit:
		return ScopeInit
	case ScopeVU:
		return ScopeVU
	default:
		return ScopeCombined
	}
}

func (s ProfileScope) captures(target ProfileScope) bool {
	switch s {
	case ScopeInit:
		return target == ScopeInit
	case ScopeVU:
		return target == ScopeVU
	default:
		return target == ScopeInit || target == ScopeVU
	}
}

// ConfigFromRuntimeOptions maps runtime options to JS observability config.
func ConfigFromRuntimeOptions(opts lib.RuntimeOptions) Config {
	cfg := Config{
		Enabled:            opts.JSProfilingEnabled.Valid && opts.JSProfilingEnabled.Bool,
		Scope:              ScopeCombined,
		MaxJSStackLabelLen: 2048,
	}
	if opts.JSProfilingScope.Valid {
		cfg.Scope = normalizeScope(opts.JSProfilingScope.String)
	}
	if opts.JSCPUProfileOutput.Valid {
		cfg.CPUProfilePath = opts.JSCPUProfileOutput.String
	}
	if opts.JSRuntimeTraceOutput.Valid {
		cfg.RuntimeTracePath = opts.JSRuntimeTraceOutput.String
	}
	if opts.JSProfileID.Valid {
		cfg.ProfileID = opts.JSProfileID.String
	}
	if opts.JSFirstRunnerMemMaxBytes.Valid {
		if v, err := parseSizeWithSuffix(opts.JSFirstRunnerMemMaxBytes.String); err == nil {
			cfg.FirstRunnerMemMaxBytes = v
		}
	}
	if opts.JSFirstRunnerMemStepPercent.Valid {
		cfg.FirstRunnerMemStepPercent = opts.JSFirstRunnerMemStepPercent.Int64
	}
	if cfg.ProfileID == "" {
		cfg.ProfileID = time.Now().UTC().Format("20060102T150405.000000000")
	}
	return cfg
}

// Artifact contains captured profile data exposed through API and files.
type Artifact struct {
	Name      string
	ProfileID string
	CreatedAt time.Time
	Data      []byte
}

// Manager controls JS observability captures for a test run.
type Manager struct {
	cfg Config

	traceBuf bytes.Buffer

	cpuOutputPath string
	traceFile     io.WriteCloser
	fs            fsext.Fs
	cpuMu         sync.Mutex
	captures      map[*sobek.Runtime]*runtimeCapture

	artifactMu sync.RWMutex
	artifacts  map[string]Artifact

	asyncTrackersMu sync.RWMutex
	asyncTrackers   map[*sobek.Runtime]*sobekAsyncContextTracker

	cpuEnabled   bool
	traceEnabled bool
	firstSampler *firstRunnerMemSampler
	firstRT      *sobek.Runtime
	mu           sync.Mutex
	stopOnce     sync.Once
}

// FirstRunnerMemMilestone represents one recorded first-runner memory threshold.
type FirstRunnerMemMilestone struct {
	ThresholdBytes uint64 `json:"threshold_bytes"`
	HeapAllocBytes uint64 `json:"heap_alloc_bytes"`
	TopFile        string `json:"top_file,omitempty"`
	TopLine        int    `json:"top_line,omitempty"`
	TopAllocSpace  int64  `json:"top_alloc_space_bytes,omitempty"`
}

// FirstRunnerMemTopLine represents one top JS line by attributed alloc_space.
type FirstRunnerMemTopLine struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	AllocSpace int64  `json:"alloc_space_bytes"`
}

// FirstRunnerMemReport is a structured view of first-runner memory sampling.
type FirstRunnerMemReport struct {
	Enabled     bool                      `json:"enabled"`
	MaxBytes    int64                     `json:"max_bytes"`
	StepPercent int64                     `json:"step_percent"`
	Milestones  []FirstRunnerMemMilestone `json:"milestones"`
	TopLines    []FirstRunnerMemTopLine   `json:"top_lines"`
}

type runtimeCapture struct {
	scope ProfileScope
	buf   bytes.Buffer
}

// NewManager builds a capture manager from config.
func NewManager(cfg Config) *Manager {
	if cfg.Scope == "" {
		cfg.Scope = ScopeCombined
	}
	if cfg.FirstRunnerMemStepPercent <= 0 {
		cfg.FirstRunnerMemStepPercent = 5
	}
	return &Manager{
		cfg:           cfg,
		fs:            fsext.NewOsFs(),
		artifacts:     make(map[string]Artifact),
		asyncTrackers: make(map[*sobek.Runtime]*sobekAsyncContextTracker),
	}
}

func parseSizeWithSuffix(v string) (int64, error) {
	var mult int64 = 1
	s := strings.TrimSpace(strings.ToLower(v))
	switch {
	case strings.HasSuffix(s, "tib"):
		mult, s = 1<<40, strings.TrimSpace(strings.TrimSuffix(s, "tib"))
	case strings.HasSuffix(s, "gib"):
		mult, s = 1<<30, strings.TrimSpace(strings.TrimSuffix(s, "gib"))
	case strings.HasSuffix(s, "mib"):
		mult, s = 1<<20, strings.TrimSpace(strings.TrimSuffix(s, "mib"))
	case strings.HasSuffix(s, "kib"):
		mult, s = 1<<10, strings.TrimSpace(strings.TrimSuffix(s, "kib"))
	case strings.HasSuffix(s, "tb"):
		mult, s = 1000*1000*1000*1000, strings.TrimSpace(strings.TrimSuffix(s, "tb"))
	case strings.HasSuffix(s, "gb"):
		mult, s = 1000*1000*1000, strings.TrimSpace(strings.TrimSuffix(s, "gb"))
	case strings.HasSuffix(s, "mb"):
		mult, s = 1000*1000, strings.TrimSpace(strings.TrimSuffix(s, "mb"))
	case strings.HasSuffix(s, "kb"):
		mult, s = 1000, strings.TrimSpace(strings.TrimSuffix(s, "kb"))
	case strings.HasSuffix(s, "b"):
		mult, s = 1, strings.TrimSpace(strings.TrimSuffix(s, "b"))
	}
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}
	base, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	if base < 0 {
		return 0, fmt.Errorf("value must be >= 0")
	}
	return base * mult, nil
}

func ensureParentDir(fs fsext.Fs, path string) error {
	parent := filepath.Dir(path)
	if parent == "." || parent == "" {
		return nil
	}
	return fs.MkdirAll(parent, 0o750)
}

// Start activates enabled captures.
func (m *Manager) Start() error {
	if !m.cfg.Enabled {
		return nil
	}
	cpuPath := m.cfg.CPUProfilePath
	tracePath := m.cfg.RuntimeTracePath
	if cpuPath == "" && tracePath == "" {
		// Keep defaults explicit when observability is enabled.
		cpuPath = "k6-js-exec-cpu.pprof"
		tracePath = "k6-js-exec.trace"
	}
	if cpuPath != "" {
		cpuPath = normalizeOutputPath(cpuPath)
		if err := ensureParentDir(m.fs, cpuPath); err != nil {
			return err
		}
		m.cpuOutputPath = cpuPath
		m.captures = make(map[*sobek.Runtime]*runtimeCapture)
		m.cpuEnabled = true
	}
	if tracePath != "" {
		tracePath = normalizeOutputPath(tracePath)
		if err := ensureParentDir(m.fs, tracePath); err != nil {
			return err
		}
		f, err := m.fs.OpenFile(tracePath, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		m.traceFile = f
		w := io.MultiWriter(f, &m.traceBuf)
		if err := rtrace.Start(w); err != nil {
			_ = f.Close()
			m.traceFile = nil
			return fmt.Errorf("start runtime trace: %w", err)
		}
		m.traceEnabled = true
	}
	return nil
}

// Stop finalizes captures and stores in-memory artifacts.
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		m.cpuMu.Lock()
		captures := m.captures
		m.captures = nil
		m.cpuMu.Unlock()
		for rt := range captures {
			rt.StopProfile()
			if rt == m.firstRT {
				rt.SetProfileSampleObserver(nil)
			}
		}
		if m.traceEnabled {
			rtrace.Stop()
			m.traceEnabled = false
		}
		if m.traceFile != nil {
			_ = m.traceFile.Close()
			m.traceFile = nil
		}
		now := time.Now().UTC()
		scopeData := m.buildScopedCPUArtifacts(captures)
		if data := scopeData[ScopeInit]; len(data) > 0 {
			m.putArtifact(Artifact{
				Name:      "js-cpu-init",
				ProfileID: m.cfg.ProfileID,
				CreatedAt: now,
				Data:      append([]byte(nil), data...),
			})
		}
		if data := scopeData[ScopeVU]; len(data) > 0 {
			m.putArtifact(Artifact{
				Name:      "js-cpu-vu",
				ProfileID: m.cfg.ProfileID,
				CreatedAt: now,
				Data:      append([]byte(nil), data...),
			})
		}
		if data := scopeData[ScopeCombined]; len(data) > 0 {
			m.putArtifact(Artifact{
				Name:      "js-cpu-combined",
				ProfileID: m.cfg.ProfileID,
				CreatedAt: now,
				Data:      append([]byte(nil), data...),
			})
		}
		if data := scopeData[m.cfg.Scope]; len(data) > 0 {
			m.putArtifact(Artifact{
				Name:      "js-cpu",
				ProfileID: m.cfg.ProfileID,
				CreatedAt: now,
				Data:      append([]byte(nil), data...),
			})
		}
		m.writeCPUArtifacts(scopeData)
		if m.traceBuf.Len() > 0 {
			m.putArtifact(Artifact{
				Name:      "js-trace",
				ProfileID: m.cfg.ProfileID,
				CreatedAt: now,
				Data:      append([]byte(nil), m.traceBuf.Bytes()...),
			})
		}
		m.reportFirstRunnerMemSampling()
	})
}

func normalizeOutputPath(path string) string {
	return filepath.Clean(strings.TrimSpace(path))
}

func (m *Manager) maybeStartRuntimeProfile(rt *sobek.Runtime, scope ProfileScope) {
	if !m.isEnabled() || !m.cpuEnabled || rt == nil || !m.cfg.Scope.captures(scope) {
		return
	}
	m.cpuMu.Lock()
	defer m.cpuMu.Unlock()
	if _, ok := m.captures[rt]; ok {
		return
	}
	if m.cfg.FirstRunnerMemMaxBytes > 0 && m.firstRT == nil {
		sampler := newFirstRunnerMemSampler(uint64(m.cfg.FirstRunnerMemMaxBytes), m.cfg.FirstRunnerMemStepPercent)
		rt.SetProfileSampleObserver(sampler.observe)
		m.firstSampler = sampler
		m.firstRT = rt
	}
	capture := &runtimeCapture{scope: scope}
	if err := rt.StartProfile(&capture.buf); err != nil {
		if rt == m.firstRT {
			rt.SetProfileSampleObserver(nil)
			m.firstRT = nil
			m.firstSampler = nil
		}
		return
	}
	m.captures[rt] = capture
}

func (m *Manager) isEnabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg.Enabled
}

// SetEnabled toggles runtime JS profiling behavior for this manager.
func (m *Manager) SetEnabled(enabled bool) {
	m.mu.Lock()
	old := m.cfg.Enabled
	m.cfg.Enabled = enabled
	m.mu.Unlock()
	if old == enabled {
		return
	}

	m.cpuMu.Lock()
	defer m.cpuMu.Unlock()
	if enabled {
		if m.captures == nil {
			m.captures = make(map[*sobek.Runtime]*runtimeCapture)
		}
		if !m.cpuEnabled {
			m.cpuEnabled = true
		}
		return
	}

	for rt := range m.captures {
		rt.StopProfile()
		rt.SetProfileSampleObserver(nil)
	}
	m.captures = make(map[*sobek.Runtime]*runtimeCapture)
	m.firstRT = nil
	m.firstSampler = nil
}

// FirstRunnerMemReport returns first-runner memory milestone and top-line data.
func (m *Manager) FirstRunnerMemReport() FirstRunnerMemReport {
	out := FirstRunnerMemReport{
		MaxBytes:    m.cfg.FirstRunnerMemMaxBytes,
		StepPercent: m.cfg.FirstRunnerMemStepPercent,
	}
	out.Enabled = m.isEnabled()
	if m.firstSampler == nil {
		return out
	}

	for _, ms := range m.firstSampler.snapshotMilestones() {
		out.Milestones = append(out.Milestones, FirstRunnerMemMilestone(ms))
	}
	for _, st := range m.firstSampler.topN(5) {
		out.TopLines = append(out.TopLines, FirstRunnerMemTopLine(st))
	}
	return out
}

func (m *Manager) reportFirstRunnerMemSampling() {
	if m.firstSampler == nil {
		return
	}
	logf := m.cfg.Logf
	if logf == nil {
		return
	}

	ms := m.firstSampler.snapshotMilestones()
	for _, mark := range ms {
		if mark.TopFile == "" {
			logf(
				"js first-runner mem milestone: threshold=%s heap=%s no-js-line-yet",
				humanizeBytes(mark.ThresholdBytes),
				humanizeBytes(mark.HeapAllocBytes),
			)
			continue
		}
		logf(
			"js first-runner mem milestone: threshold=%s heap=%s top=%s:%d alloc_space=%s",
			humanizeBytes(mark.ThresholdBytes),
			humanizeBytes(mark.HeapAllocBytes),
			mark.TopFile,
			mark.TopLine,
			humanizeBytesFromInt64(mark.TopAllocSpace),
		)
	}

	top := m.firstSampler.topN(5)
	for i, st := range top {
		logf(
			"js first-runner top alloc #%d: %s:%d alloc_space=%s",
			i+1,
			st.File,
			st.Line,
			humanizeBytesFromInt64(st.AllocSpace),
		)
	}
}

func humanizeBytesFromInt64(v int64) string {
	if v <= 0 {
		return "0B"
	}
	return humanizeBytes(uint64(v))
}

func humanizeBytes(v uint64) string {
	const unit = 1000
	if v < unit {
		return fmt.Sprintf("%dB", v)
	}
	type suffixDef struct {
		suffix string
		pow    uint64
	}
	suffixes := []suffixDef{
		{suffix: "TB", pow: unit * unit * unit * unit},
		{suffix: "GB", pow: unit * unit * unit},
		{suffix: "MB", pow: unit * unit},
		{suffix: "KB", pow: unit},
	}
	for _, s := range suffixes {
		if v >= s.pow {
			whole := v / s.pow
			frac := (v % s.pow) * 10 / s.pow
			return fmt.Sprintf("%d.%d%s", whole, frac, s.suffix)
		}
	}
	return fmt.Sprintf("%dB", v)
}

func mergeProfiles(src []*profile.Profile) []byte {
	if len(src) == 0 {
		return nil
	}
	merged, err := profile.Merge(src)
	if err != nil {
		return nil
	}
	var out bytes.Buffer
	if err := merged.Write(&out); err != nil {
		return nil
	}
	return out.Bytes()
}

func (m *Manager) buildScopedCPUArtifacts(captures map[*sobek.Runtime]*runtimeCapture) map[ProfileScope][]byte {
	scopeProfiles := map[ProfileScope][]*profile.Profile{
		ScopeInit: {},
		ScopeVU:   {},
	}
	for _, cap := range captures {
		if cap == nil || cap.buf.Len() == 0 {
			continue
		}
		pr, err := profile.Parse(bytes.NewReader(cap.buf.Bytes()))
		if err != nil {
			continue
		}
		scopeProfiles[cap.scope] = append(scopeProfiles[cap.scope], pr)
	}
	initData := mergeProfiles(scopeProfiles[ScopeInit])
	vuData := mergeProfiles(scopeProfiles[ScopeVU])
	combinedProfiles := make([]*profile.Profile, 0, len(scopeProfiles[ScopeInit])+len(scopeProfiles[ScopeVU]))
	combinedProfiles = append(combinedProfiles, scopeProfiles[ScopeInit]...)
	combinedProfiles = append(combinedProfiles, scopeProfiles[ScopeVU]...)
	combinedData := mergeProfiles(combinedProfiles)
	return map[ProfileScope][]byte{
		ScopeInit:     initData,
		ScopeVU:       vuData,
		ScopeCombined: combinedData,
	}
}

func scopePath(base string, scope ProfileScope) string {
	if base == "" {
		return ""
	}
	ext := filepath.Ext(base)
	if ext == "" {
		return base + "." + string(scope)
	}
	return strings.TrimSuffix(base, ext) + "." + string(scope) + ext
}

func (m *Manager) writeCPUArtifacts(scopeData map[ProfileScope][]byte) {
	if m.cpuOutputPath == "" {
		return
	}
	// Default output path corresponds to configured scope.
	if data := scopeData[m.cfg.Scope]; len(data) > 0 {
		_ = fsext.WriteFile(m.fs, m.cpuOutputPath, data, 0o600)
	}
	// In combined mode, also emit per-scope side files.
	if m.cfg.Scope == ScopeCombined {
		if data := scopeData[ScopeInit]; len(data) > 0 {
			_ = fsext.WriteFile(m.fs, scopePath(m.cpuOutputPath, ScopeInit), data, 0o600)
		}
		if data := scopeData[ScopeVU]; len(data) > 0 {
			_ = fsext.WriteFile(m.fs, scopePath(m.cpuOutputPath, ScopeVU), data, 0o600)
		}
	}
}

func (m *Manager) putArtifact(a Artifact) {
	m.artifactMu.Lock()
	m.artifacts[a.Name] = a
	m.artifactMu.Unlock()
}

func (m *Manager) latestArtifact(name string) (Artifact, bool) {
	m.artifactMu.RLock()
	defer m.artifactMu.RUnlock()
	v, ok := m.artifacts[name]
	return v, ok
}

// ProfileID returns correlation id for current run.
func (m *Manager) ProfileID() string {
	return m.cfg.ProfileID
}

// LabelsFromMap returns a pprof label set from non-empty key-value labels.
func LabelsFromMap(labels map[string]string) pprof.LabelSet {
	flat := make([]string, 0, len(labels)*2)
	for k, v := range labels {
		if k == "" || v == "" {
			continue
		}
		flat = append(flat, k, v)
	}
	return pprof.Labels(flat...)
}

// DoWithLabels runs fn with goroutine labels set.
func DoWithLabels(ctx context.Context, labels map[string]string, fn func(context.Context)) {
	pprof.Do(ctx, LabelsFromMap(labels), fn)
}

// WithRegion executes fn within a runtime/trace region.
func WithRegion(ctx context.Context, region string, fn func()) {
	if region == "" {
		fn()
		return
	}
	rtrace.WithRegion(ctx, region, fn)
}

// WithTask runs fn under a runtime/trace task root for hierarchy.
func WithTask(ctx context.Context, taskName string, fn func(context.Context)) {
	if taskName == "" {
		fn(ctx)
		return
	}
	taskCtx, task := rtrace.NewTask(ctx, taskName)
	defer task.End()
	fn(taskCtx)
}

//nolint:gochecknoglobals // Runtime hook sites need shared access to the currently active manager.
var active = struct {
	mu sync.RWMutex
	m  *Manager
}{}

// Activate makes manager available for runtime hook sites.
func Activate(m *Manager) {
	active.mu.Lock()
	active.m = m
	active.mu.Unlock()
}

// Deactivate removes manager from runtime hook sites.
func Deactivate(m *Manager) {
	active.mu.Lock()
	if active.m == m {
		active.m = nil
	}
	active.mu.Unlock()
}

// MaybeStartRuntimeProfile lazily starts runtime-local sobek profiling for a scope.
func MaybeStartRuntimeProfile(rt *sobek.Runtime, scope ProfileScope) {
	m := activeManager()
	if m == nil {
		return
	}
	m.maybeStartRuntimeProfile(rt, scope)
}

// CorrelationIDs returns currently active observability IDs.
func CorrelationIDs() (traceID, profileID string) {
	m := activeManager()
	if m == nil {
		return "", ""
	}
	id := m.ProfileID()
	return id, id
}

// Enabled reports whether JS observability manager is active and enabled.
func Enabled() bool {
	m := activeManager()
	return m != nil && m.isEnabled()
}

// SetProfilingEnabled toggles JS profiling at runtime.
func SetProfilingEnabled(enabled bool) bool {
	m := activeManager()
	if m == nil {
		return false
	}
	m.SetEnabled(enabled)
	return true
}

// FirstRunnerMemoryReport returns structured first-runner memory diagnostics.
func FirstRunnerMemoryReport() FirstRunnerMemReport {
	m := activeManager()
	if m == nil {
		return FirstRunnerMemReport{}
	}
	return m.FirstRunnerMemReport()
}

// LatestArtifact returns the latest stored artifact by name.
func LatestArtifact(name string) (Artifact, bool) {
	m := activeManager()
	if m == nil {
		return Artifact{}, false
	}
	return m.latestArtifact(name)
}

func activeManager() *Manager {
	active.mu.RLock()
	m := active.m
	active.mu.RUnlock()
	return m
}
