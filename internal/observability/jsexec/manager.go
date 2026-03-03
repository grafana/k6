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
	"sync"
	"time"

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
	CPUProfilePath     string
	RuntimeTracePath   string
	ProfileID          string
	MaxJSStackLabelLen int
}

// ConfigFromRuntimeOptions maps runtime options to JS observability config.
func ConfigFromRuntimeOptions(opts lib.RuntimeOptions) Config {
	cfg := Config{
		Enabled:            opts.JSProfilingEnabled.Valid && opts.JSProfilingEnabled.Bool,
		MaxJSStackLabelLen: 2048,
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

	cpuBuf   bytes.Buffer
	traceBuf bytes.Buffer

	cpuFile   *os.File
	traceFile *os.File
	cpuWriter io.Writer

	cpuEnabled   bool
	cpuStarted   bool
	traceEnabled bool
	selectedRT   *sobek.Runtime
	mu           sync.Mutex
	stopOnce     sync.Once
}

// NewManager builds a capture manager from config.
func NewManager(cfg Config) *Manager {
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
		f, err := os.Create(cpuPath)
		if err != nil {
			return err
		}
		m.cpuFile = f
		w := io.MultiWriter(f, &m.cpuBuf)
		m.cpuWriter = w
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
		if m.cpuStarted && m.selectedRT != nil {
			m.selectedRT.StopProfile()
			m.cpuStarted = false
		}
		if m.traceEnabled {
			rtrace.Stop()
			m.traceEnabled = false
		}
		if m.cpuFile != nil {
			_ = m.cpuFile.Close()
			m.cpuFile = nil
		}
		if m.traceFile != nil {
			_ = m.traceFile.Close()
			m.traceFile = nil
		}
		now := time.Now().UTC()
		if m.cpuBuf.Len() > 0 {
			putArtifact(Artifact{
				Name:      "js-cpu",
				ProfileID: m.cfg.ProfileID,
				CreatedAt: now,
				Data:      append([]byte(nil), m.cpuBuf.Bytes()...),
			})
		}
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

func (m *Manager) maybeStartRuntimeProfile(rt *sobek.Runtime) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cpuEnabled || m.cpuStarted || rt == nil {
		return
	}
	if err := rt.StartProfile(m.cpuWriter); err != nil {
		return
	}
	m.selectedRT = rt
	m.cpuStarted = true
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

// MaybeStartRuntimeProfile lazily starts runtime-local sobek profiling on first runtime seen.
func MaybeStartRuntimeProfile(rt *sobek.Runtime) {
	active.mu.RLock()
	m := active.m
	active.mu.RUnlock()
	if m == nil {
		return
	}
	m.maybeStartRuntimeProfile(rt)
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
