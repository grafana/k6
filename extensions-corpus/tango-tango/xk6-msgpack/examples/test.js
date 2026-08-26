import msgpack from "k6/x/msgpack";
import { check } from "k6";

export default function () {
  console.log("Testing k6 msgpack extension...");

  {
    const original = "hello world";
    const encoded = msgpack.pack(original);
    const decoded = msgpack.unpack(encoded);

    check(decoded, {
      "string roundtrip": (d) => d === original,
    });
    console.log("String test passed:", decoded);
  }

  {
    const original = 42;
    const encoded = msgpack.pack(original);
    const decoded = msgpack.unpack(encoded);

    check(decoded, {
      "number roundtrip": (d) => d === original,
    });
    console.log("Number test passed:", decoded);
  }

  {
    const original = [
      null,
      "ref_123",
      "channel:test",
      "phx_join",
      { user_id: 42, token: "abc123" },
    ];
    const encoded = msgpack.pack(original);
    const decoded = msgpack.unpack(encoded);

    check(decoded, {
      "phoenix message is array": (d) => Array.isArray(d),
      "phoenix message length": (d) => d.length === 5,
      "phoenix message topic": (d) => d[2] === "channel:test",
      "phoenix message event": (d) => d[3] === "phx_join",
    });
    console.log("Phoenix message test passed:", JSON.stringify(decoded));
  }

  {
    const participants = [];
    for (let i = 0; i < 10000; i++) {
      participants.push({
        id: i,
        name: `User ${i}`,
        state: "active",
      });
    }

    const original = [
      null,
      "ref_456",
      "call:abc123",
      "call:state_update",
      { participants: participants },
    ];

    const startEncode = Date.now();
    const encoded = msgpack.pack(original);
    const encodeTime = Date.now() - startEncode;

    const startDecode = Date.now();
    const decoded = msgpack.unpack(encoded);
    const decodeTime = Date.now() - startDecode;

    check(decoded, {
      "large payload is array": (d) => Array.isArray(d),
      "large payload participants count": (d) =>
        d[4].participants.length === 10000,
    });

    console.log(
      `Large payload test passed - Encode: ${encodeTime}ms, Decode: ${decodeTime}ms, Size: ${encoded.byteLength} bytes`,
    );
  }

  {
    const encoded = msgpack.pack([1, 2, 3, 4, 5]);

    check(encoded, {
      "encoded is ArrayBuffer": (e) =>
        e instanceof ArrayBuffer || e instanceof Uint8Array,
      "encoded has length": (e) => e.length > 0 || e.byteLength > 0,
    });
    console.log(
      "Binary data test passed, size:",
      encoded.length || encoded.byteLength,
      "bytes",
    );
  }

  console.log("All tests completed successfully!");
}
