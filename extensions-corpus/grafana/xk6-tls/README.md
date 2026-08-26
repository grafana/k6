# xk6-tls

A k6 extension for TLS certificates validation and inspection.

For detailed API reference and advanced usage, see the [official documentation](https://grafana.com/docs/k6/latest/javascript-api/k6-x-tls).

## What you can do

- Fetch TLS certificate information from any host
- Validate certificate expiration and properties
- Access certificate details (for example subject, issuer, fingerprint)

## Get started

Add the extension's import
```js
import tls from "k6/x/tls";
```

Call the API from the VU context
```js
import tls from "k6/x/tls";
import { check } from "k6";

export default async function () {
  const cert = await tls.getCertificate("example.com");

  check(cert, {
    "certificate is not expired": (c) => c.expires > Date.now(),
  });

  console.log(`Certificate expires: ${new Date(cert.expires)}`);
}
```

## Build

The most common and simple case is to use k6 with [automatic extension resolution](https://grafana.com/docs/k6/latest/extensions/run/#run-a-test-with-extensions). Simply add the extension's import and k6 will resolve the dependency automatically.

However, if you prefer to build it from source using xk6, first ensure you have the prerequisites:

- [Go toolchain](https://go101.org/article/go-toolchain.html)
- Git

Then:

1. Install `xk6`:
  ```shell
  go install go.k6.io/xk6/cmd/xk6@latest
  ```

2. Build the binary:
  ```shell
  xk6 build --with github.com/grafana/xk6-tls
  ```
