package appinsights

import (
	"strings"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
)

// Encapsulates contextual data common to all telemetry submitted through a
// TelemetryClient instance such as including instrumentation key, tags, and
// common properties.
type TelemetryContext struct {
	// Instrumentation key
	iKey string

	// Stripped-down instrumentation key used in envelope name
	nameIKey string

	// Collection of tag data to attach to the telemetry item.
	Tags contracts.ContextTags

	// Common properties to add to each telemetry item.  This only has
	// an effect from the TelemetryClient's context instance.  This will
	// be nil on telemetry items.
	CommonProperties map[string]string
}

// Creates a new, empty TelemetryContext
func NewTelemetryContext(ikey string) *TelemetryContext {
	return &TelemetryContext{
		iKey:             ikey,
		nameIKey:         strings.Replace(ikey, "-", "", -1),
		Tags:             make(contracts.ContextTags),
		CommonProperties: make(map[string]string),
	}
}

// Gets the instrumentation key associated with this TelemetryContext.  This
// will be an empty string on telemetry items' context instances.
func (context *TelemetryContext) InstrumentationKey() string {
	return context.iKey
}

// Wraps a telemetry item in an envelope with the information found in this
// context.
func (context *TelemetryContext) envelop(item Telemetry) *contracts.Envelope {
	// Apply common properties
	if props := item.GetProperties(); props != nil && context.CommonProperties != nil {
		for k, v := range context.CommonProperties {
			if _, ok := props[k]; !ok {
				props[k] = v
			}
		}
	}

	tdata := item.TelemetryData()
	data := contracts.NewData()
	data.BaseType = tdata.BaseType()
	data.BaseData = tdata

	envelope := contracts.NewEnvelope()
	envelope.Name = tdata.EnvelopeName(context.nameIKey)
	envelope.Data = data
	envelope.IKey = context.iKey

	timestamp := item.Time()
	if timestamp.IsZero() {
		timestamp = currentClock.Now()
	}

	envelope.Time = timestamp.UTC().Format("2006-01-02T15:04:05.999999Z")

	if contextTags := item.ContextTags(); contextTags != nil {
		envelope.Tags = contextTags

		// Copy in default tag values.
		for tagkey, tagval := range context.Tags {
			if _, ok := contextTags[tagkey]; !ok {
				contextTags[tagkey] = tagval
			}
		}
	} else {
		// Create new tags object
		envelope.Tags = make(map[string]string)
		for k, v := range context.Tags {
			envelope.Tags[k] = v
		}
	}

	// Create operation ID if it does not exist
	if _, ok := envelope.Tags[contracts.OperationId]; !ok {
		envelope.Tags[contracts.OperationId] = newUUID().String()
	}

	// Sanitize.
	for _, warn := range tdata.Sanitize() {
		diagnosticsWriter.Printf("Telemetry data warning: %s", warn)
	}
	for _, warn := range contracts.SanitizeTags(envelope.Tags) {
		diagnosticsWriter.Printf("Telemetry tag warning: %s", warn)
	}

	return envelope
}
