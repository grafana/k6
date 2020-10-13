package appinsights

import (
	"net/http"
	"os"
	"runtime"
	"time"
)

// Configuration data used to initialize a new TelemetryClient.
type TelemetryConfiguration struct {
	// Instrumentation key for the client.
	InstrumentationKey string

	// Endpoint URL where data will be submitted.
	EndpointUrl string

	// Maximum number of telemetry items that can be submitted in each
	// request.  If this many items are buffered, the buffer will be
	// flushed before MaxBatchInterval expires.
	MaxBatchSize int

	// Maximum time to wait before sending a batch of telemetry.
	MaxBatchInterval time.Duration

	// Customized http client if desired (will use http.DefaultClient otherwise)
	Client *http.Client
}

// Creates a new TelemetryConfiguration object with the specified
// instrumentation key and default values.
func NewTelemetryConfiguration(instrumentationKey string) *TelemetryConfiguration {
	return &TelemetryConfiguration{
		InstrumentationKey: instrumentationKey,
		EndpointUrl:        "https://dc.services.visualstudio.com/v2/track",
		MaxBatchSize:       1024,
		MaxBatchInterval:   time.Duration(10) * time.Second,
	}
}

func (config *TelemetryConfiguration) setupContext() *TelemetryContext {
	context := NewTelemetryContext(config.InstrumentationKey)
	context.Tags.Internal().SetSdkVersion(sdkName + ":" + Version)
	context.Tags.Device().SetOsVersion(runtime.GOOS)

	if hostname, err := os.Hostname(); err == nil {
		context.Tags.Device().SetId(hostname)
		context.Tags.Cloud().SetRoleInstance(hostname)
	}

	return context
}
