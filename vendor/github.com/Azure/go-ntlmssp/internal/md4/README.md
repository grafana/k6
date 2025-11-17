# MD4 Implementation

This package contains an identical copy of the MD4 hash implementation from Go's extended cryptography package (`golang.org/x/crypto/md4`).

## Why Vendored?

This MD4 implementation is vendored locally to avoid depending on the `golang.org/x/crypto` package, which can introduce version conflicts and dependency management issues in `go.mod`. By maintaining our own copy, we ensure:

- **Stability**: No external dependency version conflicts
- **Simplicity**: Cleaner `go.mod` file without xcrypto dependency
- **Control**: Full control over the implementation without external changes

## Source

The original implementation can be found at:
- Package: `golang.org/x/crypto/md4`
- Repository: https://github.com/golang/crypto

## Usage

This package is intended for internal use within the go-ntlmssp library only. The MD4 hash algorithm is required for NTLM authentication but should not be used for general cryptographic purposes as MD4 is considered cryptographically broken.
