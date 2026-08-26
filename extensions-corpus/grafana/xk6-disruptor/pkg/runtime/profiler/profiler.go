// Package profiler offers functions to profile the execution of a process
// using go's built-in profiling tools
package profiler

import (
	"io"
)

// Config is the configuration of the profiler
type Config struct {
	CPU     CPUConfig
	Memory  MemoryConfig
	Metrics MetricsConfig
	Trace   TraceConfig
}

// Profiler defines the methods to control execution profiling
type Profiler interface {
	// Start stars the collection of profiling information with the given configuration
	Start(config Config) (io.Closer, error)
}

// Probe defines the interface for controlling a profiling probe
type Probe interface {
	Start() (io.Closer, error)
}

// profiler maintains the configuration state of the profiler
type profiler struct {
	closers []io.Closer
}

// NewProfiler creates a Profiler instance
func NewProfiler() Profiler {
	return &profiler{}
}

// Start stars the collection of profiling information with the given configuration
func (p *profiler) Start(config Config) (io.Closer, error) {
	probes, err := buildProbes(config)
	if err != nil {
		return nil, err
	}

	closers := []io.Closer{}
	for _, probe := range probes {
		closer, err := probe.Start()
		if err != nil {
			// ensure any started Probe is closed
			_ = p.Close()
			return nil, err
		}

		closers = append(closers, closer)
	}

	p.closers = closers

	return p, nil
}

// Stops the collection of  profiling data and sends it to the output files
func (p *profiler) Close() error {
	// TODO: report error(s)
	for _, c := range p.closers {
		_ = c.Close()
	}
	return nil
}

func buildProbes(config Config) ([]Probe, error) {
	probes := []Probe{}

	if config.CPU.Enabled {
		probe, err := NewCPUProbe(config.CPU)
		if err != nil {
			return nil, err
		}
		probes = append(probes, probe)
	}

	if config.Memory.Enabled {
		probe, err := NewMemoryProbe(config.Memory)
		if err != nil {
			return nil, err
		}
		probes = append(probes, probe)
	}

	if config.Trace.Enabled {
		probe, err := NewTraceProbe(config.Trace)
		if err != nil {
			return nil, err
		}
		probes = append(probes, probe)
	}

	if config.Metrics.Enabled {
		probe, err := NewMetricsProbe(config.Metrics)
		if err != nil {
			return nil, err
		}
		probes = append(probes, probe)
	}

	return probes, nil
}
