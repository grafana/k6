package provisioning

import (
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/lib/types"
)

// Config holds the configuration needed to construct a provisioning Client.
type Config struct {
	Token   null.String        `json:"token"`
	Host    null.String        `json:"host"`
	Timeout types.NullDuration `json:"timeout"`
}

// Apply saves config non-zero config values from the passed config in the receiver.
func (c Config) Apply(cfg Config) Config {
	if cfg.Token.Valid {
		c.Token = cfg.Token
	}
	if cfg.Host.Valid && cfg.Host.String != "" {
		c.Host = cfg.Host
	}
	if cfg.Timeout.Valid {
		c.Timeout = cfg.Timeout
	}
	return c
}
