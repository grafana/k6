// Loads a page from a server that requires a TLS client certificate (mutual TLS).
//
// CERT_SOURCE selects where the certificate comes from (default: file):
//
//   file         PEM files next to this script, read with open() in the init context:
//                  ORIGIN=https://localhost:8443 k6 run client_certificates.js
//   secrets-pem  base64-encoded PEM certificate and key from k6/secrets (the file secret
//                source holds one key=value per line, so PEM must be base64-encoded):
//                  client-cert=<base64 PEM>
//                  client-key=<base64 PEM>
//                  client-key-passphrase=<passphrase>  (optional; for an encrypted key)
//                  ORIGIN=https://localhost:8443 CERT_SOURCE=secrets-pem \
//                    k6 run --secret-source=file=client.secrets client_certificates.js
//
// Set IGNORE_HTTPS_ERRORS=true only for a test server whose certificate the browser does not
// trust (e.g. self-signed); it disables verification of the server, see the security notes.
//
// Differences from Playwright:
// - At most one certificate per key type (RSA, ECDSA, Ed25519) per origin; duplicates are
//   rejected.
// - PFX (pfx), socks5:// context proxies for https, and HTTP(S)_PROXY/NO_PROXY are not
//   supported yet; use PEM cert and key, and an http:// or https:// context proxy.
// - No certPath or keyPath: read files with open() in the init context instead.
// - Encrypted PKCS#8 keys are not supported; use an unencrypted or legacy-encrypted PEM key.
// - DataView values are rejected; pass a string, ArrayBuffer or typed array.
// - Origins that are not https:// or cannot be parsed are rejected.
// - proxy.bypass is applied by Chromium, so a bypassed origin gets no client certificate.
// - host-resolver-rules from K6_BROWSER_ARGS do not apply in a context with clientCertificates,
//   whose connections go through a k6 proxy; use the k6 hosts option instead.
// - Remote browsers (K6_BROWSER_WS_URL, chromium.connectOverCDP) are not supported.
// - Error pages say "k6 client-certificate error".
//
// Security notes:
// - A context with clientCertificates ignores certificate errors for every origin, as in
//   Playwright; k6 still verifies the server for origins listed in clientCertificates.
// - TLS 1.2 sends the client certificate in cleartext.
// - With ignoreHTTPSErrors: true the client certificate can be presented to an impostor server.
// - A server with optional client authentication may reject a certificate it does not accept,
//   where it would have accepted no certificate at all.
import { browser } from 'k6/browser';
import { fail } from 'k6';
import { SharedArray } from 'k6/data';
import encoding from 'k6/encoding';
import secrets from 'k6/secrets';
import { check } from 'https://jslib.k6.io/k6-utils/1.5.0/index.js';

const ORIGIN = __ENV.ORIGIN || 'https://localhost:8443';
const CERT_SOURCE = __ENV.CERT_SOURCE || 'file';

// open() works only in the init context; SharedArray keeps one copy for all VUs.
const files = CERT_SOURCE === 'file'
  ? new SharedArray('client certificate', () => [{
      cert: open('./client.crt'),
      key: open('./client.key'),
    }])
  : null;

export const options = {
  scenarios: {
    ui: {
      executor: 'shared-iterations',
      options: {
        browser: {
            type: 'chromium',
        },
      },
    },
  },
  thresholds: {
    checks: ["rate==1.0"]
  }
}

async function optionalSecret(key) {
  try {
    return await secrets.get(key);
  } catch (_) {
    return undefined;
  }
}

async function clientCertificate() {
  switch (CERT_SOURCE) {
  case 'file':
    return { origin: ORIGIN, cert: files[0].cert, key: files[0].key };
  case 'secrets-pem':
    return {
      origin: ORIGIN,
      cert: encoding.b64decode(await secrets.get('client-cert'), 'std', 's'),
      key: encoding.b64decode(await secrets.get('client-key'), 'std', 's'),
      passphrase: await optionalSecret('client-key-passphrase'),
    };
  default:
    throw new Error(`unknown CERT_SOURCE ${CERT_SOURCE}; use file or secrets-pem`);
  }
}

export default async function() {
  const context = await browser.newContext({
    clientCertificates: [await clientCertificate()],
    ignoreHTTPSErrors: __ENV.IGNORE_HTTPS_ERRORS === 'true',
  });
  const page = await context.newPage();

  try {
    const response = await page.goto(ORIGIN);

    await check(response, {
      'status is 200': r => r.status() === 200,
    });
  } catch (error) {
    fail(`Browser iteration failed: ${error.message}`);
  } finally {
    await page.close();
  }
}
