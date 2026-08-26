// Package kafka implements the k6/x/kafka extension: the official,
// Grafana-owned, pure-Go k6 extension for load testing Apache Kafka.
//
// This file provides the module scaffold — registration, the exported constant
// surface, and the public symbols (Writer, Reader, Connection, SchemaRegistry,
// LoadJKS). Method behavior is implemented by later changes; see index.d.ts for
// the authoritative API contract.
package kafka

import (
	"fmt"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modules"
)

// RootModule is the global module factory; one is created per k6 process.
type RootModule struct{}

// Module is the per-VU instance of the k6/x/kafka module.
type Module struct {
	vu      modules.VU
	exports *sobek.Object
	metrics *kafkaMetrics
}

var (
	_ modules.Module   = (*RootModule)(nil)
	_ modules.Instance = (*Module)(nil)
)

// NewModuleInstance implements modules.Module. It builds the module's default
// export object with the flat constants and the public symbols.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	m := &Module{vu: vu, exports: vu.Runtime().NewObject()}
	// Register custom metrics from the init environment (present when the module
	// is imported during VU init). If absent, metrics are simply not emitted.
	if ie := vu.InitEnv(); ie != nil && ie.Registry != nil {
		m.metrics = registerMetrics(ie.Registry)
	}
	m.defineConstants()
	m.defineSymbols()
	return m
}

// writerCollector / readerCollector build a per-client metrics collector, or nil
// when metrics are unavailable (no registry).
func (m *Module) writerCollector() *metricsCollector {
	if m.metrics == nil {
		return nil
	}
	return newMetricsCollector(m.metrics, roleWriter)
}

func (m *Module) readerCollector() *metricsCollector {
	if m.metrics == nil {
		return nil
	}
	return newMetricsCollector(m.metrics, roleReader)
}

// Exports implements modules.Instance. The module members are exposed as the
// default export object; named imports resolve to its properties.
func (m *Module) Exports() modules.Exports {
	return modules.Exports{Default: m.exports}
}

// defineConstants attaches the flat top-level constants to the export object.
func (m *Module) defineConstants() {
	rt := m.vu.Runtime()
	for name, value := range moduleConstants() {
		if err := m.exports.Set(name, value); err != nil {
			common.Throw(rt, err)
		}
	}
}

// defineSymbols registers the public constructors and the LoadJKS function.
// Construction succeeds; method behavior is delivered by later changes.
func (m *Module) defineSymbols() {
	rt := m.vu.Runtime()
	set := func(name string, value any) {
		if err := m.exports.Set(name, value); err != nil {
			common.Throw(rt, err)
		}
	}

	// Writer/Reader/Connection are implemented by their changes. SchemaRegistry
	// and LoadJKS are implemented by this change.
	set("Writer", m.newWriter)
	set("Reader", m.newReader)
	set("Connection", m.newConnection)
	set("SchemaRegistry", m.newSchemaRegistry)
	set("LoadJKS", m.loadJKS)
}

// newWriter constructs a Writer: decode the config and build a producer client.
// The returned object exposes the instance methods (produce, close).
func (m *Module) newWriter(call sobek.ConstructorCall) *sobek.Object {
	rt := m.vu.Runtime()

	var cfg WriterConfig
	if len(call.Arguments) > 0 {
		if err := rt.ExportTo(call.Argument(0), &cfg); err != nil {
			common.Throw(rt, fmt.Errorf("invalid writer config: %w", err))
		}
	}

	writer, err := openWriter(m.vu, cfg, m.writerCollector())
	if err != nil {
		common.Throw(rt, err)
	}
	return rt.ToValue(writer).ToObject(rt)
}

// newReader constructs a Reader: decode the config and build a consumer client.
// The returned object exposes the instance methods (consume, close).
func (m *Module) newReader(call sobek.ConstructorCall) *sobek.Object {
	rt := m.vu.Runtime()

	var cfg ReaderConfig
	if len(call.Arguments) > 0 {
		if err := rt.ExportTo(call.Argument(0), &cfg); err != nil {
			common.Throw(rt, fmt.Errorf("invalid reader config: %w", err))
		}
	}

	reader, err := openReader(m.vu, cfg, m.readerCollector())
	if err != nil {
		common.Throw(rt, err)
	}
	return rt.ToValue(reader).ToObject(rt)
}

// newConnection constructs a Connection: it decodes the config, builds an
// authenticated client, and verifies connectivity (failing on an unreachable
// cluster). The returned object exposes the instance methods (e.g. close).
func (m *Module) newConnection(call sobek.ConstructorCall) *sobek.Object {
	rt := m.vu.Runtime()

	var cfg ConnectionConfig
	if len(call.Arguments) > 0 {
		if err := rt.ExportTo(call.Argument(0), &cfg); err != nil {
			common.Throw(rt, fmt.Errorf("invalid connection config: %w", err))
		}
	}

	conn, err := openConnection(m.vu, cfg)
	if err != nil {
		common.Throw(rt, err)
	}
	return rt.ToValue(conn).ToObject(rt)
}

// newSchemaRegistry constructs a SchemaRegistry: it decodes the config,
// creates an authenticated HTTP client (with optional TLS), and validates
// connectivity via the /config endpoint. The returned object exposes the
// instance methods (e.g. serialize, deserialize, getSchema, createSchema).
func (m *Module) newSchemaRegistry(call sobek.ConstructorCall) *sobek.Object {
	rt := m.vu.Runtime()

	var cfg *SchemaRegistryConfig
	if len(call.Arguments) > 0 {
		var c SchemaRegistryConfig
		if err := rt.ExportTo(call.Argument(0), &c); err != nil {
			common.Throw(rt, fmt.Errorf("invalid schema registry config: %w", err))
		}
		cfg = &c
	}

	sr, err := NewSchemaRegistry(m.vu, cfg)
	if err != nil {
		common.Throw(rt, err)
	}
	return rt.ToValue(sr).ToObject(rt)
}
