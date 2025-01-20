package common

import "time"

const (
	// Defaults

	DefaultLocale       string        = "en-US"
	DefaultScreenWidth  int64         = 1280
	DefaultScreenHeight int64         = 720
	DefaultTimeout      time.Duration = 30 * time.Second

	// Life-cycle consts

	LifeCycleNetworkIdleTimeout time.Duration = 500 * time.Millisecond

	// API default consts.

	StrictModeOff = false
)
