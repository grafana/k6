[**xk6-kafka**](../README.md)

---

# Function: LoadJKS()

> **LoadJKS**(`jksConfig`): [`JKS`](../interfaces/JKS.md)

Defined in: [index.d.ts:558](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/index.d.ts#L558)

**`Function`**

## Parameters

### jksConfig

[`JKSConfig`](../interfaces/JKSConfig.md)

JKS configuration.

## Returns

[`JKS`](../interfaces/JKS.md)

- JKS client and server certificates and private key.

## Description

Load a JKS keystore from a file.

## Example

```javascript
const jks = LoadJKS({
  path: "/path/to/keystore.jks",
  password: "password",
  clientCertAlias: "localhost",
  clientKeyAlias: "localhost",
  clientKeyPassword: "password",
  serverCaAlias: "ca",
});
```
