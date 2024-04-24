# xk6-webcrypto

This is a **work in progress** project implementation of the [WebCrypto API](https://developer.mozilla.org/en-US/docs/Web/API/Web_Crypto_API) specification for k6.

## Current state

The current state of the project is that it is an experimental module of the WebCrypto API specification. While we consider it ready for production use, it is still missing some features and is not yet fully compliant with the specification.

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
| `crypto.subtle.encrypt()` | ✅      | ✅      | ✅      | ❌       |
| `crypto.subtle.decrypt()` | ✅      | ✅      | ✅      | ❌       |

##### Signature

| API                      | HMAC | ECDSA | RSASSA-PKCS1-v1_5 | RSA-PSS |
| :----------------------- | :--- | :---- | :---------------- | :------ |
| `crypto.subtle.sign()`   | ✅   | ✅    | ❌                | ❌      |
| `crypto.subtle.verify()` | ✅   | ✅    | ❌                | ❌      |

##### Key generation, import and export

| API                           | AES-CBC | AES-GCM | AES-CTR | AES-KW | HMAC | ECDSA | ECDH | RSASSA-PKCS1-v1_5 | RSA-PSS | RSA-OAEP |
| :---------------------------- | :------ | :------ | :------ | :----- | :--- | :---- | :--- | :---------------- | :------ | :------- |
| `crypto.subtle.generateKey()` | ✅      | ✅      | ✅      | ❌     | ✅   | ✅    | ✅   | ❌                | ❌      | ❌       |
| `crypto.subtle.importKey()`   | ✅      | ✅      | ✅      | ❌     | ✅   | ✅    | ✅   | ❌                | ❌      | ❌       |
| `crypto.subtle.exportKey()`   | ✅      | ✅      | ✅      | ❌     | ✅   | ✅    | ✅   | ❌                | ❌      | ❌       |

> [!WARNING]  
> Currently, only the `raw` and `jwk` (JSON Web Key) formats are supported for import/export operations for the `AES-*` and `HMAC` algorithms. `ECDH` and `ECDSA` have support for `pkcs8`, `spki`, `raw` and `jwk` formats.

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

- **AES-KW**: in the current state of things, this module does not support the AES-KW (JSON Key Wrap) algorithm. The reason for it is that the Go standard library does not support it. We are looking into alternatives, but for now, this is a limitation of the module.
- **AES-GCM**: although the algorithm is supported, and can already be used, it is not fully compliant with the specification. The reason for this is that the Go standard library only supports a 12-byte nonce/iv, while the specification allows for a wider range of sizes. We do not expect to address this limitation unless the Go standard library adds support for it.

## Contributing

Contributions are welcome! If the module is missing a feature you need, or if you find a bug, please open an issue or a pull request. If you are not sure about something, feel free to open an issue and ask.

### Practices

The [WebCrypto API specification](https://www.w3.org/TR/WebCryptoAPI) is quite large, and it is not always easy to understand what is going on. To help with that, we have adopted a few practices that we hope will make it easier for contributors to understand the codebase.

#### Algorithm steps numbered comments

Contributors will likely notice that the codebase is annotated with comments of the form `// {some number}.`. Those comments are used to track the progress of the implementation of the specification and to ensure the correctness of the implementation of the algorithms. The numbers are the section numbers of the specification. For example, the comment `// 8.` in the `SubtleCrypto.GenerateKey` function refers to [step 8 of the `generateKey` algorithm from the specification](https://www.w3.org/TR/WebCryptoAPI/#SubtleCrypto-method-generateKey).

Following this convention allows us to document why certain operations are made in a certain way, and to track the progress of the implementation. We do not always add them, but we try to do so when it makes sense, and encourage contributors to do the same.
