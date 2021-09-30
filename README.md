# xk6-output-prometheus-remote

K6 extension for Prometheus remote write output. **Alpha version!**

*Distinguish from [Prometheus remote write **client** extension](https://github.com/grafana/xk6-client-prometheus-remote) :)*

According to [Prometheus API Stability Guarantees](https://prometheus.io/docs/prometheus/latest/stability/) remote write is an **experimental feature**, thus it is unstable and is subject to change as of now.

To enable remote write in Prometheus 2.x use `--enable-feature=remote-write-receiver` option. See docker-compose samples in `example/`. Options for remote write storage can be found [here](https://prometheus.io/docs/operating/integrations/). The exact details of how metrics will be processed or stored may depend on the underlying storage used.

Important points to know:

- remote write format does not contain info about metric types
- remote read may not work without precise queries; see [here](https://prometheus.io/docs/prometheus/latest/storage/#remote-storage-integrations) and [here](https://github.com/timescale/promscale/issues/64) for details and example
- some third party storages may support additional formats, like JSON, but it is not part of official Prometheus remote write specification and therefore absent here
- exemplars are not yet officially supported by format, see [here](https://github.com/prometheus/prometheus/issues/9317)

### Usage

To build k6 binary with the Prometheus remote write output extension use:

```
xk6 build --with github.com/grafana/xk6-output-prometheus-remote=. 
```

Then run new k6 binary with:

```
K6_PROMETHEUS_REMOTE_URL=http://localhost:9090/api/v1/write ./k6 run script.js -o output-prometheus-remote
```

Add TLS and HTTP basic authentication:

```
K6_PROMETHEUS_REMOTE_URL=https://localhost:9090/api/v1/write K6_PROMETHEUS_INSECURE_SKIP_TLS_VERIFY=false K6_CA_CERT_FILE=example/tls.crt K6_PROMETHEUS_USER=foo K6_PROMETHEUS_PASSWORD=bar ./k6 run script.js -o output-prometheus-remote
```

Note: Prometheus remote client relies on a snappy library for serialization which can panic on [encode operation](https://github.com/golang/snappy/blob/544b4180ac705b7605231d4a4550a1acb22a19fe/encode.go#L22).

### On sample rate

K6 processes its outputs once per second and that is also a default flush period in this extension. The number of K6 builtin metrics is 26 as of now and they are collected at the rate of 50ms. In practice it means that there will be around 1000-1500 sample on average per each flush period. If custom metrics are configured, that estimate will have to be adjusted.

Depending on exact Prometheus setup, it may be necessary to configure Prometheus and / or remote storage to handle the load. Specifically, see [`queue_config` parameter](https://prometheus.io/docs/practices/remote_write/) of Prometheus.

### Next steps

- [ ] decide on the specification. Some questions:
   - [ ] additional config options?
   - [ ] since there can be differences in behaviour between storages, should there be a maintained list of what is used / tested by K6 team?
   - [ ] should metric types be supported and if so, how (from user's perspective)?
   - [ ] exemplars may soon be supported by Prometheus officially: would anyone need them?
- [ ] add some details / examples about labels