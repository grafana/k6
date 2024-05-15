package opentelemetry

import (
	"fmt"
	"time"

	"go.k6.io/k6/output"
)

// Config is the config for the template collector
type Config struct {
	Address      string
	PushInterval time.Duration
}

// NewConfig creates a new Config instance from the provided output.Params
func NewConfig(p output.Params) (Config, error) {
	cfg := Config{
		Address:      "template",
		PushInterval: 1 * time.Second,
	}

	for k, v := range p.Environment {
		switch k {
		case "K6_TEMPLATE_PUSH_INTERVAL":
			var err error
			cfg.PushInterval, err = time.ParseDuration(v)
			if err != nil {
				return cfg, fmt.Errorf("error parsing environment variable 'K6_TEMPLATE_PUSH_INTERVAL': %w", err)
			}
		case "K6_TEMPLATE_ADDRESS":
			cfg.Address = v
		}
	}
	return cfg, nil
}
