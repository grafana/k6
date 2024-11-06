# xk6-webcrypto

This is an implementation of the [WebCrypto API](https://developer.mozilla.org/en-US/docs/Web/API/Web_Crypto_API) specification for k6.

## Current state

Starting the version 0.44.0, this implementation is available in k6 as an experimental module. To use it, you need to import it in your script.

```javascript
import webcrypto from 'k6/experimental/webcrypto';
```

While we consider it ready for production use, it is still missing some features and is not yet fully compliant with the specification.

### Supported APIs and algorithms

#### Crypto

| API                        | Supported | Notes        |
| :------------------------- | :-------- | :----------- |
| `crypto.getRandomValues()` | ✅        | **complete** |
| `crypto.randomUUID()`      | ✅        | **complete** |

#### SubtleCrypto


##### Encryption/Decryption

| API                       | AES-CBC | AES-GCM | AES-CTR | RSA-OAEP |
| :------------------------ | :------ | :------ | :------ | :------- |
| `crypto.subtle.encrypt()` | ✅      | ✅      | ✅      | ✅       |
| `crypto.subtle.decrypt()` | ✅      | ✅      | ✅      | ✅       |

##### Signature

| API                      | HMAC | ECDSA | RSASSA-PKCS1-v1_5 | RSA-PSS |
| :----------------------- | :--- | :---- | :---------------- | :------ |
| `crypto.subtle.sign()`   | ✅   | ✅    | ✅                | ✅      |
| `crypto.subtle.verify()` | ✅   | ✅    | ✅                | ✅      |

##### Key generation, import, and export

| API                           | AES-CBC | AES-GCM | AES-CTR | AES-KW | HMAC | ECDSA | ECDH | RSASSA-PKCS1-v1_5 | RSA-PSS | RSA-OAEP |
| :---------------------------- | :------ | :------ | :------ | :----- | :--- | :---- | :--- | :---------------- | :------ | :------- |
| `crypto.subtle.generateKey()` | ✅      | ✅      | ✅      | ❌     | ✅   | ✅    | ✅   | ✅                | ✅      | ✅       |
| `crypto.subtle.importKey()`   | ✅      | ✅      | ✅      | ❌     | ✅   | ✅    | ✅   | ✅                | ✅      | ✅       |
| `crypto.subtle.exportKey()`   | ✅      | ✅      | ✅      | ❌     | ✅   | ✅    | ✅   | ✅                | ✅      | ✅       |

> [!WARNING]  
> Currently, only the `raw` and `jwk` (JSON Web Key) formats are supported for import/export operations for the `AES-*` and `HMAC` algorithms. `ECDH` and `ECDSA` have support for `pkcs8`, `spki`, `raw` and `jwk` formats. RSA algorithms have support for `pkcs8`, `spki`  and `jwk` formats.

##### Key derivation

| API                          | ECDH | HKDF | PBKDF2 |
| :--------------------------- | :--- | :--- | :----- |
| `crypto.subtle.deriveKey()`  | ❌   | ❌   | ❌     |
| `crypto.subtle.deriveBits()` | ✅   | ❌   | ❌     |

Note: `deriveBits` currently doesn't support length parameter non-multiple of 8.

##### Key wrapping

| API                         | AES-CBC | AES-GCM | AES-CTR | AES-KW | RSA-OAEP |
| :-------------------------- | :------ | :------ | :------ | :----- | :------- |
| `crypto.subtle.unwrapKey()` | ❌      | ❌      | ❌      | ❌     | ❌       |

### APIs and algorithms with limited support

- **WebCrypto API support**: Even though we implemented the vast majority of the WebCrypto API, there are still some APIs and algorithms that are not yet supported. By looking at the [WebCrypto API Compliant](https://github.com/grafana/xk6-webcrypto/issues?q=is%3Aissue+state%3Aopen+label%3A%22WebCrypto+API+Compliant%22) label, you can see what is missing.
- **AES-GCM**: Although the algorithm is supported, and can already be used, it is not fully compliant with the specification. The reason for this is that the Go standard library only supports a 12-byte nonce/iv, while the specification allows for a wider range of sizes. We do not expect to address this limitation unless the Go standard library adds support for it.
- **RSA-PSS**: Since we use Golang SDK under the hood, the RSA-PSS [doesn't support deterministic signatures](https://github.com/golang/go/blob/master/src/crypto/rsa/pss.go#L293-L297). In other words, even if `saltLength` is set to 0, the signature will be different each time.

## Contributing

Contributions are welcome! If the module is missing a feature you need, or if you find a bug, please open an issue or a pull request. If you are not sure about something, feel free to open an issue and ask.

### Practices

In our implementation we rely on [WebCrypto API specification](https://www.w3.org/TR/WebCryptoAPI). Sometimes it feels quite large, and it is not always easy to understand what is going on. To help with that, we have adopted a few practices that we hope will make it easier for contributors to understand the codebase.

Following this convention allows us to document why certain operations are made in a certain way, and to track the progress of the implementation. We do not always add them, but we try to do so when it makes sense, and encourage contributors to do the same.

### Web Platform Tests

We aim to be compliant with the WebCrypto API specification. To ensure that, we test our implementation against the [Web Platform Tests](https://web-platform-tests.org/), this is a part of the CI, and it's expected to implement the missing tests when it's needed. See the [webcrypto/tests/README.md](./webcrypto/tests/README.md) for more details.