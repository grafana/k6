// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jaeger

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	jaegerHeader        = "uber-trace-id"
	separator           = ":"
	traceID64bitsWidth  = 64 / 4
	traceID128bitsWidth = 128 / 4
	spanIDWidth         = 64 / 4

	traceIDPadding = "0000000000000000"

	flagsDebug      = 0x02
	flagsSampled    = 0x01
	flagsNotSampled = 0x00

	deprecatedParentSpanID = "0"
)

var (
	empty = trace.SpanContext{}

	errMalformedTraceContextVal = errors.New("header value of uber-trace-id should contain four different part separated by : ")
	errInvalidTraceIDLength     = errors.New("invalid trace id length, must be either 16 or 32")
	errMalformedTraceID         = errors.New("cannot decode trace id from header, should be a string of hex, lowercase trace id can't be all zero")
	errInvalidSpanIDLength      = errors.New("invalid span id length, must be 16")
	errMalformedSpanID          = errors.New("cannot decode span id from header, should be a string of hex, lowercase span id can't be all zero")
	errMalformedFlag            = errors.New("cannot decode flag")
)

// Jaeger propagator serializes SpanContext to/from Jaeger Headers
//
// Jaeger format:
//
// uber-trace-id: {trace-id}:{span-id}:{parent-span-id}:{flags}
type Jaeger struct{}

var _ propagation.TextMapPropagator = &Jaeger{}

// Inject injects a context to the carrier following jaeger format.
// The parent span ID is set to an dummy parent span id as the most implementations do.
func (jaeger Jaeger) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	sc := trace.SpanFromContext(ctx).SpanContext()
	headers := []string{}
	if !sc.TraceID.IsValid() || !sc.SpanID.IsValid() {
		return
	}
	headers = append(headers, sc.TraceID.String(), sc.SpanID.String(), deprecatedParentSpanID)
	if sc.IsDebug() {
		headers = append(headers, fmt.Sprintf("%x", flagsDebug|flagsSampled))
	} else if sc.IsSampled() {
		headers = append(headers, fmt.Sprintf("%x", flagsSampled))
	} else {
		headers = append(headers, fmt.Sprintf("%x", flagsNotSampled))
	}

	carrier.Set(jaegerHeader, strings.Join(headers, separator))
}

// Extract extracts a context from the carrier if it contains Jaeger headers.
func (jaeger Jaeger) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	// extract tracing information
	if h := carrier.Get(jaegerHeader); h != "" {
		sc, err := extract(h)
		if err == nil && sc.IsValid() {
			return trace.ContextWithRemoteSpanContext(ctx, sc)
		}
	}

	return ctx
}

func extract(headerVal string) (trace.SpanContext, error) {
	var (
		sc  = trace.SpanContext{}
		err error
	)

	parts := strings.Split(headerVal, separator)
	if len(parts) != 4 {
		return empty, errMalformedTraceContextVal
	}

	// extract trace ID
	if parts[0] != "" {
		id := parts[0]
		if len(id) != traceID128bitsWidth && len(id) != traceID64bitsWidth {
			return empty, errInvalidTraceIDLength
		}
		// padding when length is 16
		if len(id) == traceID64bitsWidth {
			id = traceIDPadding + id
		}
		sc.TraceID, err = trace.TraceIDFromHex(id)
		if err != nil {
			return empty, errMalformedTraceID
		}
	}

	// extract span ID
	if parts[1] != "" {
		id := parts[1]
		if len(id) != spanIDWidth {
			return empty, errInvalidSpanIDLength
		}
		sc.SpanID, err = trace.SpanIDFromHex(id)
		if err != nil {
			return empty, errMalformedSpanID
		}
	}

	// skip third part as it is deprecated

	// extract flag
	if parts[3] != "" {
		flagStr := parts[3]
		flag, err := strconv.ParseInt(flagStr, 16, 64)
		if err != nil {
			return empty, errMalformedFlag
		}
		if flag&flagsSampled == flagsSampled {
			// if sample bit is set, we check if debug bit is also set
			if flag&flagsDebug == flagsDebug {
				sc.TraceFlags |= trace.FlagsSampled | trace.FlagsDebug
			} else {
				sc.TraceFlags |= trace.FlagsSampled
			}
		}
		// ignore other bit, including firehose since we don't have corresponding flag in trace context.
	}
	return sc, nil
}

func (jaeger Jaeger) Fields() []string {
	return []string{jaegerHeader}
}
