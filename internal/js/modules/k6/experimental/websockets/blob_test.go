package websockets

import (
	"fmt"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
)

func TestBlob(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		blobPartsDef  string
		bytesExpected []byte
	}{
		"String": {
			blobPartsDef:  `["PASS"]`,
			bytesExpected: []byte("PASS"),
		},
		"MultipleStrings": {
			blobPartsDef:  `["P", "A", "SS"]`,
			bytesExpected: []byte("PASS"),
		},
		"ArrayBuffer": {
			blobPartsDef:  `[new Uint8Array([0x50, 0x41, 0x53, 0x53]).buffer]`,
			bytesExpected: []byte("PASS"),
		},
		"Int8Array": {
			blobPartsDef:  `[new Int8Array([0x50, 0x41, 0x53, 0x53])]`,
			bytesExpected: []byte("PASS"),
		},
		"Uint8Array": {
			blobPartsDef:  `[new Uint8Array([0x50, 0x41, 0x53, 0x53])]`,
			bytesExpected: []byte("PASS"),
		},
		"Uint8ClampedArray": {
			blobPartsDef:  `[new Uint8ClampedArray([0x50, 0x41, 0x53, 0x53])]`,
			bytesExpected: []byte("PASS"),
		},
		"Int16Array": {
			blobPartsDef:  `[new Int16Array([0x4150, 0x5353])]`,
			bytesExpected: []byte("PASS"),
		},
		"Uint16Array": {
			blobPartsDef:  `[new Uint16Array([0x4150, 0x5353])]`,
			bytesExpected: []byte("PASS"),
		},
		"Int32Array": {
			blobPartsDef:  `[new Int32Array([0x53534150])]`,
			bytesExpected: []byte("PASS"),
		},
		"Uint32Array": {
			blobPartsDef:  `[new Uint32Array([0x53534150])]`,
			bytesExpected: []byte("PASS"),
		},
		"Float32Array": {
			blobPartsDef:  `[new Float32Array(new Uint8Array([0x50, 0x41, 0x53, 0x53]).buffer)]`,
			bytesExpected: []byte("PASS"),
		},
		"Float64Array": {
			// Byte length of Float64Array should be a multiple of 8
			blobPartsDef:  `[new Float64Array(new Uint8Array([0x50, 0x41, 0x53, 0x53, 0x00, 0x00, 0x00, 0x00]).buffer)]`,
			bytesExpected: append([]byte("PASS"), 0x0, 0x0, 0x0, 0x0),
		},
		"DataView": {
			blobPartsDef:  `[new DataView(new Int8Array([0x50, 0x41, 0x53, 0x53]).buffer)]`,
			bytesExpected: []byte("PASS"),
		},
		"Blob": {
			blobPartsDef:  `[new Blob(["PASS"])]`,
			bytesExpected: []byte("PASS"),
		},
	}

	for name, tc := range tcs {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ts := newTestState(t)
			val, err := ts.runtime.RunOnEventLoop(fmt.Sprintf(`
				(async () => {
					const blobParts = %s;
					const blob = new Blob(blobParts);
		  			return blob.arrayBuffer();
				})()
			`, tc.blobPartsDef))

			require.NoError(t, err)

			p, ok := val.Export().(*sobek.Promise)
			require.True(t, ok)

			ab, ok := p.Result().Export().(sobek.ArrayBuffer)
			require.True(t, ok)
			require.Equal(t, tc.bytesExpected, ab.Bytes())
		})
	}
}

func TestBlob_type(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	rt := ts.runtime.VU.Runtime()
	val, err := ts.runtime.RunOnEventLoop(`
		new Blob(["PASS"], { type: "text/example" });
	`)
	require.NoError(t, err)
	require.True(t, isBlob(val.ToObject(rt), ts.module.blobConstructor))
	require.Equal(t, "text/example", val.ToObject(rt).Get("type").String())
}

func TestBlob_size(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	rt := ts.runtime.VU.Runtime()
	val, err := ts.runtime.RunOnEventLoop(`
		new Blob(["PASS"]);
	`)
	require.NoError(t, err)
	require.True(t, isBlob(val.ToObject(rt), ts.module.blobConstructor))
	require.Equal(t, int64(4), val.ToObject(rt).Get("size").ToInteger())
}

func TestBlob_arrayBuffer(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	val, err := ts.runtime.RunOnEventLoop(`
		(async () => {
			const blob = new Blob(["P", "A", "SS"]);
			return blob.arrayBuffer();
		})()
	`)
	require.NoError(t, err)

	p, ok := val.Export().(*sobek.Promise)
	require.True(t, ok)

	ab, ok := p.Result().Export().(sobek.ArrayBuffer)
	require.True(t, ok)
	require.Equal(t, []byte("PASS"), ab.Bytes())
}

func TestBlob_bytes(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	val, err := ts.runtime.RunOnEventLoop(`
		(async () => {
			const blob = new Blob(["P", "A", "SS"]);
			return blob.bytes();
		})()
	`)
	require.NoError(t, err)

	p, ok := val.Export().(*sobek.Promise)
	require.True(t, ok)

	rt := ts.runtime.VU.Runtime()
	res := p.Result().ToObject(rt)
	require.True(t, isUint8Array(res, rt))

	var resBytes []byte
	require.NoError(t, rt.ExportTo(res.Get("buffer"), &resBytes))
	require.Equal(t, []byte("PASS"), resBytes)
}

func TestBlob_slice(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		call          string
		bytesExpected []byte
		ctExpected    string
	}{
		"slice()": {
			call:          `slice()`,
			bytesExpected: []byte("PASS"),
		},
		"slice(start)": {
			call:          `slice(1)`,
			bytesExpected: []byte("ASS"),
		},
		"slice(start, end)": {
			call:          `slice(0,1)`,
			bytesExpected: []byte("P"),
		},
		"slice(start, end, contentType)": {
			call:          `slice(0,1,"text/example")`,
			bytesExpected: []byte("P"),
			ctExpected:    "text/example",
		},
	}

	for name, tc := range tcs {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ts := newTestState(t)
			val, err := ts.runtime.RunOnEventLoop(fmt.Sprintf(`
				(async () => {
					const blob = new Blob(["PASS"]);
					return blob.%s;
				})()
			`, tc.call))

			require.NoError(t, err)

			p, ok := val.Export().(*sobek.Promise)
			require.True(t, ok)

			rt := ts.runtime.VU.Runtime()
			assertBlobTypeAndContents(t, ts, p.Result().ToObject(rt), tc.ctExpected, tc.bytesExpected)
		})
	}
}

func assertBlobTypeAndContents(t *testing.T, ts testState, blob *sobek.Object, expType string, expContents []byte) {
	t.Helper()

	// First, we assert the given object is 'instanceof' Blob.
	require.True(t, isBlob(blob, ts.module.blobConstructor))

	// Then, we assert the type of the blob.
	require.Equal(t, expType, blob.Get("type").String())

	// Finally, we assert the contents of the blob, by calling `.arrayBuffer()`
	// and comparing the result with the expected contents.
	call, ok := sobek.AssertFunction(blob.Get("arrayBuffer"))
	require.True(t, ok)

	ret, err := call(sobek.Undefined())
	require.NoError(t, err)
	p, ok := ret.Export().(*sobek.Promise)
	require.True(t, ok)

	ab, ok := p.Result().Export().(sobek.ArrayBuffer)
	require.True(t, ok)
	require.Equal(t, expContents, ab.Bytes())
}

func TestBlob_text(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	val, err := ts.runtime.RunOnEventLoop(`
		(async () => {
			const blob = new Blob(["P", "A", "SS"]);
			return blob.text();
		})()
	`)
	require.NoError(t, err)

	p, ok := val.Export().(*sobek.Promise)
	require.True(t, ok)

	require.Equal(t, "PASS", p.Result().String())
}

func TestBlob_stream(t *testing.T) {
	t.Parallel()
	ts := newTestState(t)
	val, err := ts.runtime.RunOnEventLoop(`
		(async () => {
		  const blob = new Blob(["P", "A", "SS"]);
		  const reader = blob.stream().getReader();
		  const {value} = await reader.read();
		  return value;
		})()
	`)
	require.NoError(t, err)

	p, ok := val.Export().(*sobek.Promise)
	require.True(t, ok)
	require.Equal(t, "PASS", p.Result().String())
}
