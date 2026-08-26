package profiler

import (
	"fmt"
	"io"
	"os"
	"runtime/trace"
)

// TraceConfig defines the configuration of a tracing probe
type TraceConfig struct {
	Enabled  bool
	FileName string
}

type traceProbe struct {
	config TraceConfig
	file   *os.File
}

// NewTraceProbe creates a trace profiling probe with the given configuration
func NewTraceProbe(config TraceConfig) (Probe, error) {
	if config.FileName == "" {
		return nil, fmt.Errorf("trace output file name cannot be empty")
	}

	return &traceProbe{
		config: config,
	}, nil
}

func (t *traceProbe) Start() (io.Closer, error) {
	var err error

	t.file, err = os.Create(t.config.FileName)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace output file %q: %w", t.config.FileName, err)
	}

	if err := trace.Start(t.file); err != nil {
		return nil, fmt.Errorf("failed to start trace: %w", err)
	}

	return t, nil
}

func (t *traceProbe) Close() error {
	trace.Stop()
	return t.file.Close()
}
