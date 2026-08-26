# xk6-msgpack

A k6 extension for high-performance MessagePack encoding and decoding using
https://github.com/vmihailenco/msgpack

## Building

To build k6 with this extension, you need xk6:

```bash
go install go.k6.io/xk6/cmd/xk6@latest
```

Then build k6 with the msgpack extension:

```bash
xk6 build --with github.com/tangotango/xk6-msgpack=.
```

This will create a `k6` binary in the current directory with the msgpack
extension included.

## Usage

```javascript
import msgpack from "k6/x/msgpack";

export default function () {
  // Encode data to MessagePack
  const data = [null, "ref123", "topic", "event", { key: "value" }];
  const encoded = msgpack.pack(data);

  // Decode MessagePack data
  const decoded = msgpack.unpack(encoded);
  console.log(decoded);
}
```

## Examples

```bash
./k6 run examples/test.js
```

### output

```
         /\      Grafana   /‾‾/
    /\  /  \     |\  __   /  /
   /  \/    \    | |/ /  /   ‾‾\
  /          \   |   (  |  (‾)  |
 / __________ \  |_|\_\  \_____/

     execution: local
        script: examples/test.js
        output: -

     scenarios: (100.00%) 1 scenario, 1 max VUs, 10m30s max duration (incl. graceful stop):
              * default: 1 iterations for each of 1 VUs (maxDuration: 10m0s, gracefulStop: 30s)

INFO[0000] Testing k6 msgpack extension...               source=console
INFO[0000] String test passed: hello world               source=console
INFO[0000] Number test passed: 42                        source=console
INFO[0000] Phoenix message test passed: [null,"ref_123","channel:test","phx_join",{"user_id":42,"token":"abc123"}]  source=console
INFO[0000] Large payload test passed - Encode: 4ms, Decode: 3ms, Size: 408947 bytes  source=console
INFO[0000] Binary data test passed, size: 46 bytes       source=console
INFO[0000] All tests completed successfully!             source=console


  █ TOTAL RESULTS

    checks_total.......: 10      814.232458/s
    checks_succeeded...: 100.00% 10 out of 10
    checks_failed......: 0.00%   0 out of 10

    ✓ string roundtrip
    ✓ number roundtrip
    ✓ phoenix message is array
    ✓ phoenix message length
    ✓ phoenix message topic
    ✓ phoenix message event
    ✓ large payload is array
    ✓ large payload participants count
    ✓ encoded is ArrayBuffer
    ✓ encoded has length

    EXECUTION
    iteration_duration...: avg=12.23ms min=12.23ms med=12.23ms max=12.23ms p(90)=12.23ms p(95)=12.23ms
    iterations...........: 1   81.423246/s

    NETWORK
    data_received........: 0 B 0 B/s
    data_sent............: 0 B 0 B/s




running (00m00.0s), 0/1 VUs, 1 complete and 0 interrupted iterations
default ✓ [======================================] 1 VUs  00m00.0s/10m0s  1/1 iters, 1 per VU
```
