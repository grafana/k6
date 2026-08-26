# Kerberos Authentication

## Background
The bundled `librdkafka` is built *without* Kerberos support. To use Kerberos with 
`xk6-kafka`, `xk6-kafka` must be built and linked with an `librdkafka` build with 
Cyrus SASL support.

## Pre-Requisites
- `librdkafka` built with Cyrus SASL support

Many distribution-provided builds of `librdkafka` include Cyrus SASL support.

### Fedora
```bash
dnf install librdkafka librdkafka-devel
```

## Building xk6-kafka
When building your `xk6` binary, include the dynamic build flags tag: `--build-flags="-tags=dynamic"`

Example:
```shell
CGO_ENABLED=1 xk6 build --with github.com/mostafa/xk6-kafka/v2@latest --build-flags="-tags=dynamic"
```

Example for local development:
```shell
CGO_ENABLED=1 xk6 build --with github.com/mostafa/xk6-kafka/v2@latest=. --build-flags="-tags=dynamic"
```

You can verify that `librdkafka` and `libsasl2` are dynamically 
linked in with `ldd k6`:
```
librdkafka.so.1 => /lib64/librdkafka.so.1 (0x00007f7caf600000)
libsasl2.so.3 => /lib64/libsasl2.so.3 (0x00007f7caf8f8000)
```

## Configuring xk6-kafka
Kerberos is configured using the `kerberos` configuration object 
within the `SASL` config object.

The following optional configurations are available:

| Property | Description |
| -------- | ----------- |
| serviceName | Kerberos principal name that Kafka runs as |
| principal | This client's Kerberos principal name |
| kInitCmd | Shell command to refresh or acquire the client's Kerberos ticket |
| keyTab | Path to Kerberos keytab file |
| minTimeBeforeRelogin | Minimum time in milliseconds between key refresh attempts |

If a configuration is undefined, it will use the `librdkafka` defaults.

Example:
```typescript
const saslConfig = {
  algorithm: SASL_GSSAPI,
  kerberosConfig: {
    serviceName: "kafka",
    principal: "test@example.com",
    kInitCmd: "kinit",
    keyTab: "/path/to/keytab",
    minTimeBeforeRelogin: 1000,
  }
};
```

Example with defaults:
```typescript
const saslConfig = {
  algorithm: SASL_GSSAPI,
}
```

See [`../scripts/test_sasl_kerberos.js`](../scripts/test_sasl_kerberos.js) for a complete example.
