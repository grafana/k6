package profiler

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/metrics"
	"time"
)

// MetricsConfig defines the configuration of a metrics probe
type MetricsConfig struct {
	Enabled  bool
	FileName string
	Rate     time.Duration
}

type metricsProbe struct {
	config MetricsConfig
	cancel context.CancelFunc
}

// NewMetricsProbe creates a metrics profiling probe with the given configuration
func NewMetricsProbe(config MetricsConfig) (Probe, error) {
	if config.FileName == "" {
		return nil, fmt.Errorf("metrics output file name cannot be empty")
	}

	return &metricsProbe{
		config: config,
	}, nil
}

func (m *metricsProbe) Start() (io.Closer, error) {
	metricsFile, err := os.Create(m.config.FileName)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics output file %q: %w", m.config.FileName, err)
	}

	collector := &metricsCollector{
		metricsFile: metricsFile,
		rate:        m.config.Rate,
	}

	ctx, cancel := context.WithCancel(context.Background())
	err = collector.Start(ctx)
	if err != nil {
		cancel()
		return nil, err
	}

	m.cancel = cancel
	return m, nil
}

func (m *metricsProbe) Close() error {
	// stops the collector
	m.cancel()

	return nil
}

type stats struct {
	count  uint
	minval float64
	maxval float64
	total  float64
}

func (s *stats) add(value float64) {
	// if first sample, use value as min and max
	if s.count == 0 || value < s.minval {
		s.minval = value
	}
	if s.count == 0 || value > s.maxval {
		s.maxval = value
	}
	s.total += value
	s.count++
}

func (s *stats) avg() float64 {
	// avoid division by 0
	if s.count == 0 {
		return 0
	}
	return s.total / float64(s.count)
}

func (s *stats) min() float64 {
	return s.minval
}

func (s *stats) max() float64 {
	return s.maxval
}

// metricsCollector maintains the state for collecting metrics
type metricsCollector struct {
	rate        time.Duration
	samples     []metrics.Sample
	metricsFile *os.File
	stats       map[string]*stats
}

// Start starts the periodic metrics collection.
// When the context is cancelled, it generates a summary to the metrics file
func (m *metricsCollector) Start(ctx context.Context) error {
	m.init()

	// start periodic sampling in background
	go func() {
		ticks := time.NewTicker(m.rate)
		defer ticks.Stop()

		for {
			select {
			case <-ticks.C:
				m.sample()
			case <-ctx.Done():
				m.generate()
				return
			}
		}
	}()

	return nil
}

func (m *metricsCollector) init() {
	m.stats = map[string]*stats{}

	for _, metric := range metrics.All() {
		// skip histogram values
		if metric.Kind == metrics.KindUint64 || metric.Kind == metrics.KindFloat64 {
			m.samples = append(
				m.samples,
				metrics.Sample{
					Name: metric.Name,
				},
			)
			m.stats[metric.Name] = &stats{}
		}
	}

	m.sample()
}

func (m *metricsCollector) sample() {
	metrics.Read(m.samples)
	for _, sample := range m.samples {
		stats := m.stats[sample.Name]

		var value float64
		switch sample.Value.Kind() {
		case metrics.KindFloat64:
			value = sample.Value.Float64()
		case metrics.KindUint64:
			value = float64(sample.Value.Uint64())
		default:
			continue
		}

		stats.add(value)
	}
}

func (m *metricsCollector) generate() {
	_, _ = fmt.Fprintln(m.metricsFile, "metric,min,max,average")
	for k, v := range m.stats {
		_, _ = fmt.Fprintf(m.metricsFile, "%s,%.2f,%.2f,%.2f\n", k, v.min(), v.max(), v.avg())
	}
	_ = m.metricsFile.Close()
}
