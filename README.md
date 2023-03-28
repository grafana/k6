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

| API                                                                           | Supported | Notes                                                    |
| :---------------------------------------------------------------------------- | :-------- | :------------------------------------------------------- |
| `crypto.subtle.digest(algorithm)`                                             | ✅        | **complete**                                             |
| `crypto.subtle.generateKey(algorithm, extractable, keyUsages)`                | ✅        | **limited to** AES-CBC, AES-GCM, and AES-CTR algorithms. |
| `crypto.subtle.importKey(format, keyData, algorithm, extractable, keyUsages)` | ✅        | **limited to** AES-CBC, AES-GCM, and AES-CTR algorithms. |
| `crypto.subtle.exportKey(format, key)`                                        | ✅        | **limited to** AES-CBC, AES-GCM, and AES-CTR algorithms. |
| `crypto.subtle.encrypt(algorithm, key, data)`                                 | ✅        | **limited to** AES-CBC, AES-GCM, and AES-CTR algorithms. |
| `crypto.subtle.decrypt(algorithm, key, data)`                                 | ✅        | **limited to** AES-CBC, AES-GCM, and AES-CTR algorithms. |
| `crypto.subtle.deriveBits()`                                                  | ❌        |                                                          |
| `crypto.subtle.deriveKey()`                                                   | ❌        |                                                          |
| `crypto.subtle.sign()`                                                        | ❌        |                                                          |
| `crypto.subtle.verify()`                                                      | ❌        |                                                          |
| `crypto.subtle.wrapKey()`                                                     | ❌        |                                                          |
| `crypto.subtle.unwrapKey()`                                                   | ❌        |                                                          |


### APIs and algorithms with limited support

- **AES-KW**: in the current state of things, this module does not support the AES-KW (JSON Key Wrap) algorithm. The reason for it is that the Go standard library does not support it. We are looking into alternatives, but for now, this is a limitation of the module.
- **AES-GCM**: although the algorithm is supported, and can already be used, it is not fully compliant with the specification. The reason for this is that the Go standard library only supports a 12-byte nonce/iv, while the specification allows for a wider range of sizes. We do not expect to address this limitation unless the Go standard library adds support for it.

## Contributing

Contributions are welcome!

### Practices

Contributors will likely notice that the codebase is annotated with comments of the form `// {some number}.`. Those comments are used to track the progress of the implementation of the specification and to ensure the correctness of the implementation of the algorithms. The numbers are the section numbers of the specification. For example, the comment `// 8.` in the `SubtleCrypto.GenerateKey` function refers to [step 8 of the `generateKey` algorithm from the specification](https://www.w3.org/TR/WebCryptoAPI/#SubtleCrypto-method-generateKey).

Following this convention allows us to document why certain operations are made in a certain way, and to track the progress of the implementation. We do not always add them, but we try to do so when it makes sense, and encourage contributors to do the same.
