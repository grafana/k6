package kafka

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
)

// newTestModule builds a module instance on a test runtime and exposes its
// default export as the global `kafka`.
func newTestModule(t *testing.T) *modulestest.Runtime {
	t.Helper()
	rt := modulestest.NewRuntime(t)
	mi := new(RootModule).NewModuleInstance(rt.VU)
	require.NoError(t, rt.VU.Runtime().Set("kafka", mi.Exports().Default))
	return rt
}

func TestModuleExportsAndConstruction(t *testing.T) {
	t.Parallel()

	rt := newTestModule(t)
	_, err := rt.VU.Runtime().RunString(`
		// Scaffold constructors still present and construct without error.
		// (Connection now connects eagerly and needs a broker, so it is covered
		// by the gated integration tests, not here.)
		new kafka.Writer({ brokers: ["localhost:9092"], topic: "t" });
		new kafka.Reader({ brokers: ["localhost:9092"], topic: "t" });
		new kafka.SchemaRegistry();

		// LoadJKS is present as a function.
		if (typeof kafka.LoadJKS !== "function") {
			throw new Error("LoadJKS is not a function");
		}

		// Flat constants carry the contract values.
		if (kafka.CODEC_SNAPPY !== "snappy") throw new Error("CODEC_SNAPPY");
		if (kafka.KEY !== "key" || kafka.VALUE !== "value") throw new Error("element types");
		if (kafka.SECOND !== 1000000000) throw new Error("SECOND");

		// Grouped values are flat, not enum objects.
		if (typeof kafka.COMPRESSION_CODECS !== "undefined") {
			throw new Error("COMPRESSION_CODECS should not be exported");
		}
	`)
	require.NoError(t, err)
}

func TestSchemaRegistryJSBridgeSerdes(t *testing.T) {
	t.Parallel()

	rt := newTestModule(t)
	_, err := rt.VU.Runtime().RunString(`
		function asBytes(v) {
			if (v instanceof ArrayBuffer) {
				return new Uint8Array(v);
			}
			return v;
		}

		const sr = new kafka.SchemaRegistry();

		const encodedString = sr.serialize({
			data: "hello world",
			schemaType: "STRING",
		});
		const decodedString = sr.deserialize({
			data: encodedString,
			schemaType: "STRING",
		});
		if (decodedString !== "hello world") {
			throw new Error("string round-trip failed");
		}

		const encodedBytes = sr.serialize({
			data: new Uint8Array([72, 101, 108, 108, 111]),
			schemaType: "BYTES",
		});
		const decodedBytes = asBytes(sr.deserialize({
			data: encodedBytes,
			schemaType: "BYTES",
		}));
		if (String.fromCharCode.apply(null, decodedBytes) !== "Hello") {
			throw new Error("bytes round-trip failed");
		}

		const encodedAvro = sr.serialize({
			data: { id: 1, name: "Alice" },
			schemaType: "AVRO",
			schema: {
				schema: JSON.stringify({
					type: "record",
					name: "User",
					fields: [
						{ name: "id", type: "int" },
						{ name: "name", type: "string" },
					],
				}),
				schemaType: "AVRO",
			},
		});
		const decodedAvro = sr.deserialize({
			data: encodedAvro,
			schemaType: "AVRO",
			schema: {
				schema: JSON.stringify({
					type: "record",
					name: "User",
					fields: [
						{ name: "id", type: "int" },
						{ name: "name", type: "string" },
					],
				}),
				schemaType: "AVRO",
			},
		});
		if (decodedAvro.id !== 1 || decodedAvro.name !== "Alice") {
			throw new Error("avro round-trip failed");
		}
	`)
	require.NoError(t, err)
}
