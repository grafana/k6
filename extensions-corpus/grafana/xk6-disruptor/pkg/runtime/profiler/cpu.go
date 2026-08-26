package profiler

import (
	"fmt"
	"io"
	"os"
	"runtime/pprof"
)

// CPUConfig defines the configuration of a CPU profiling probe
type CPUConfig struct {
	Enabled  bool
	FileName string
}

// cpuProbe maintains the state of a CPU Probe
type cpuProbe struct {
	config CPUConfig
	file   *os.File
}

// NewCPUProbe creates a new CPU profiling probe
func NewCPUProbe(config CPUConfig) (Probe, error) {
	if config.FileName == "" {
		return nil, fmt.Errorf("CPU profile file name cannot be empty")
	}

	return &cpuProbe{
		config: config,
	}, nil
}

func (c *cpuProbe) Start() (io.Closer, error) {
	var err error

	c.file, err = os.Create(c.config.FileName)
	if err != nil {
		return nil, fmt.Errorf("error creating CPU profiling file %q: %w", c.config.FileName, err)
	}

	err = pprof.StartCPUProfile(c.file)
	if err != nil {
		return nil, fmt.Errorf("failed to start CPU profiling: %w", err)
	}

	return c, nil
}

func (c *cpuProbe) Close() error {
	pprof.StopCPUProfile()
	return nil
}
