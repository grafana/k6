package disruptors

import (
	"context"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/types/intstr"
)

// ProtocolFaultInjector defines the methods for injecting protocol faults
type ProtocolFaultInjector interface {
	// InjectHTTPFault injects faults in the HTTP requests sent to the disruptor's targets
	// for the specified duration
	InjectHTTPFaults(ctx context.Context, fault HTTPFault, duration time.Duration, options HTTPDisruptionOptions) error
	// InjectGrpcFault injects faults in the grpc requests sent to the disruptor's targets
	// for the specified duration
	InjectGrpcFaults(ctx context.Context, fault GrpcFault, duration time.Duration, options GrpcDisruptionOptions) error
}

// HTTPDisruptionOptions defines options for the injection of HTTP faults in a target pod
type HTTPDisruptionOptions struct {
	// Port used by the agent for listening
	ProxyPort uint `js:"proxyPort"`
}

// GrpcDisruptionOptions defines options for the injection of grpc faults in a target pod
type GrpcDisruptionOptions struct {
	// Port used by the agent for listening
	ProxyPort uint `js:"proxyPort"`
}

// HTTPFault specifies a fault to be injected in http requests
type HTTPFault struct {
	// port the disruptions will be applied to
	Port intstr.IntOrString
	// Average delay introduced to requests
	AverageDelay time.Duration `js:"averageDelay"`
	// Variation in the delay (with respect of the average delay)
	DelayVariation time.Duration `js:"delayVariation"`
	// Fraction (in the range 0.0 to 1.0) of requests that will return an error
	ErrorRate float32 `js:"errorRate"`
	// Error code to be returned by requests selected in the error rate
	ErrorCode uint `js:"errorCode"`
	// Body to be returned when an error is injected
	ErrorBody string `js:"errorBody"`
	// Comma-separated list of url paths to be excluded from disruptions
	Exclude string
}

// GrpcFault specifies a fault to be injected in grpc requests
type GrpcFault struct {
	// port the disruptions will be applied to
	Port intstr.IntOrString
	// Average delay introduced to requests
	AverageDelay time.Duration `js:"averageDelay"`
	// Variation in the delay (with respect of the average delay)
	DelayVariation time.Duration `js:"delayVariation"`
	// Fraction (in the range 0.0 to 1.0) of requests that will return an error
	ErrorRate float32 `js:"errorRate"`
	// Status code to be returned by requests selected to return an error
	StatusCode int32 `js:"statusCode"`
	// Status message to be returned in requests selected to return an error
	StatusMessage string `js:"statusMessage"`
	// List of grpc services to be excluded from disruptions
	Exclude string `js:"exclude"`
}
