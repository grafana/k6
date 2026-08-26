package profiler

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
)

// MemoryConfig defines the configuration of a Memory profiling probe
type MemoryConfig struct {
	Enabled  bool
	FileName string
	Rate     int
}

// memoryProbe maintains the state of a memory probe
type memoryProbe struct {
	config MemoryConfig
	file   *os.File
}

// NewMemoryProbe creates a memory profiling probe with the given configuration
func NewMemoryProbe(config MemoryConfig) (Probe, error) {
	if config.Rate < 0 {
		return nil, fmt.Errorf("memory rate must be non-negative: %d", config.Rate)
	}

	if config.FileName == "" {
		return nil, fmt.Errorf("memory profile file name cannot be empty")
	}

	return &memoryProbe{
		config: config,
	}, nil
}

func (m *memoryProbe) Start() (io.Closer, error) {
	var err error
	m.file, err = os.Create(m.config.FileName)
	if err != nil {
		return nil, fmt.Errorf("error creating memory profiling file %q: %w", m.config.FileName, err)
	}

	runtime.MemProfileRate = m.config.Rate

	return m, nil
}

func (m *memoryProbe) Close() error {
	err := pprof.Lookup("heap").WriteTo(m.file, 0)
	if err != nil {
		return fmt.Errorf("failed to write memory profile to file %q: %w", m.config.FileName, err)
	}

	return nil
}
