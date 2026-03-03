package jsexec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/pprof"
	rtrace "runtime/trace"
	"strings"
	"sync"
	"time"

	"github.com/google/pprof/profile"
	"github.com/grafana/sobek"

	"go.k6.io/k6/lib"
)

const (
	// Metadata keys that can be propagated to outputs for correlation.
	MetadataTraceIDKey   = "js_trace_id"
	MetadataProfileIDKey = "js_profile_id"
)

// Config defines JS observability capture settings.
type Config struct {
	Enabled            bool
	Scope              ProfileScope
	CPUProfilePath     string
	RuntimeTracePath   string
	ProfileID          string
	MaxJSStackLabelLen int
}

type ProfileScope string

const (
	ScopeInit     ProfileScope = "init"
	ScopeVU       ProfileScope = "vu"
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

var artifactStore = struct {
	mu   sync.RWMutex
	data map[string]Artifact
}{
	data: make(map[string]Artifact),
}

// LatestArtifact returns the latest stored artifact by name.
func LatestArtifact(name string) (Artifact, bool) {
	artifactStore.mu.RLock()
	defer artifactStore.mu.RUnlock()
	v, ok := artifactStore.data[name]
	return v, ok
}

func putArtifact(a Artifact) {
	artifactStore.mu.Lock()
	artifactStore.data[a.Name] = a
	artifactStore.mu.Unlock()
}

// Manager controls JS observability captures for a test run.
type Manager struct {
	cfg Config

	traceBuf bytes.Buffer

	cpuOutputPath string
	traceFile     *os.File
	cpuMu         sync.Mutex
	captures      map[*sobek.Runtime]*runtimeCapture

	cpuEnabled   bool
	traceEnabled bool
	mu           sync.Mutex
	stopOnce     sync.Once
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
	return &Manager{cfg: cfg}
}

func ensureParentDir(path string) error {
	parent := filepath.Dir(path)
	if parent == "." || parent == "" {
		return nil
	}
	return os.MkdirAll(parent, 0o755)
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
		if err := ensureParentDir(cpuPath); err != nil {
			return err
		}
		m.cpuOutputPath = cpuPath
		m.captures = make(map[*sobek.Runtime]*runtimeCapture)
		m.cpuEnabled = true
	}
	if tracePath != "" {
		if err := ensureParentDir(tracePath); err != nil {
			return err
		}
		f, err := os.Create(tracePath)
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
			putArtifact(Artifact{
				Name:      "js-cpu-init",
				ProfileID: m.cfg.ProfileID,
				CreatedAt: now,
				Data:      append([]byte(nil), data...),
			})
		}
		if data := scopeData[ScopeVU]; len(data) > 0 {
			putArtifact(Artifact{
				Name:      "js-cpu-vu",
				ProfileID: m.cfg.ProfileID,
				CreatedAt: now,
				Data:      append([]byte(nil), data...),
			})
		}
		if data := scopeData[ScopeCombined]; len(data) > 0 {
			putArtifact(Artifact{
				Name:      "js-cpu-combined",
				ProfileID: m.cfg.ProfileID,
				CreatedAt: now,
				Data:      append([]byte(nil), data...),
			})
		}
		if data := scopeData[m.cfg.Scope]; len(data) > 0 {
			putArtifact(Artifact{
				Name:      "js-cpu",
				ProfileID: m.cfg.ProfileID,
				CreatedAt: now,
				Data:      append([]byte(nil), data...),
			})
		}
		m.writeCPUArtifacts(scopeData)
		if m.traceBuf.Len() > 0 {
			putArtifact(Artifact{
				Name:      "js-trace",
				ProfileID: m.cfg.ProfileID,
				CreatedAt: now,
				Data:      append([]byte(nil), m.traceBuf.Bytes()...),
			})
		}
	})
}

func (m *Manager) maybeStartRuntimeProfile(rt *sobek.Runtime, scope ProfileScope) {
	if !m.cpuEnabled || rt == nil || !m.cfg.Scope.captures(scope) {
		return
	}
	m.cpuMu.Lock()
	defer m.cpuMu.Unlock()
	if _, ok := m.captures[rt]; ok {
		return
	}
	capture := &runtimeCapture{scope: scope}
	if err := rt.StartProfile(&capture.buf); err != nil {
		return
	}
	m.captures[rt] = capture
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
		_ = os.WriteFile(m.cpuOutputPath, data, 0o644)
	}
	// In combined mode, also emit per-scope side files.
	if m.cfg.Scope == ScopeCombined {
		if data := scopeData[ScopeInit]; len(data) > 0 {
			_ = os.WriteFile(scopePath(m.cpuOutputPath, ScopeInit), data, 0o644)
		}
		if data := scopeData[ScopeVU]; len(data) > 0 {
			_ = os.WriteFile(scopePath(m.cpuOutputPath, ScopeVU), data, 0o644)
		}
	}
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
	active.mu.RLock()
	m := active.m
	active.mu.RUnlock()
	if m == nil {
		return
	}
	m.maybeStartRuntimeProfile(rt, scope)
}

// CorrelationIDs returns currently active observability IDs.
func CorrelationIDs() (traceID, profileID string) {
	active.mu.RLock()
	m := active.m
	active.mu.RUnlock()
	if m == nil {
		return "", ""
	}
	id := m.ProfileID()
	return id, id
}

// Enabled reports whether JS observability manager is active and enabled.
func Enabled() bool {
	active.mu.RLock()
	m := active.m
	active.mu.RUnlock()
	return m != nil && m.cfg.Enabled
}
