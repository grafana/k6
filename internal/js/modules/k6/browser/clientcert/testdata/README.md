# Test fixtures

The PKCS#12 files in this directory contain synthetic, self-signed keys and certificates
generated for tests only. They protect nothing and must never be used outside tests.
All files use the passphrase `k6-test`.

| File | Contents |
|---|---|
| `rsa-3des.p12` | RSA leaf `CN=k6-test-pfx-rsa`, legacy 3DES/RC2 + SHA-1 MAC |
| `ecdsa-3des.p12` | ECDSA P-256 leaf `CN=k6-test-pfx-ecdsa`, legacy 3DES/RC2 + SHA-1 MAC |
| `rsa-aes.p12` | RSA leaf `CN=k6-test-pfx-rsa`, OpenSSL default AES-256 + SHA-256 MAC |
| `chain-3des.p12` | RSA leaf `CN=k6-test-pfx-chain` signed by `CN=k6-test-ca`, bundled with the CA, legacy |

Generated with OpenSSL:

```bash
openssl req -x509 -newkey rsa:2048 -nodes -subj /CN=k6-test-pfx-rsa -days 36500 -keyout rsa.key -out rsa.crt
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -subj /CN=k6-test-pfx-ecdsa -days 36500 -keyout ec.key -out ec.crt
openssl pkcs12 -export -legacy -inkey rsa.key -in rsa.crt -passout pass:k6-test -out rsa-3des.p12
openssl pkcs12 -export -legacy -inkey ec.key  -in ec.crt  -passout pass:k6-test -out ecdsa-3des.p12
openssl pkcs12 -export          -inkey rsa.key -in rsa.crt -passout pass:k6-test -out rsa-aes.p12
openssl req -x509 -newkey rsa:2048 -nodes -subj /CN=k6-test-ca -days 36500 -keyout ca.key -out ca.crt
openssl req -newkey rsa:2048 -nodes -subj /CN=k6-test-pfx-chain -keyout chain.key -out chain.csr
openssl x509 -req -in chain.csr -CA ca.crt -CAkey ca.key -days 36500 -out chain.crt
openssl pkcs12 -export -legacy -inkey chain.key -in chain.crt -certfile ca.crt -passout pass:k6-test -out chain-3des.p12
rm -f *.key *.crt *.csr
```
