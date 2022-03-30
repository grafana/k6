# xk6-output-prometheus-remote
k6 extension for publishing test-run metrics to Prometheus via Remote Write endpoint.

> :warning: Not to be confused with [Prometheus Remote Write **client** extension](https://github.com/grafana/xk6-client-prometheus-remote) which is for load testing _Prometheus_ itself.

There are many options for remote-write compatible agents, the official list can be found [here](https://prometheus.io/docs/operating/integrations/). The exact details of how metrics will be processed or stored depends on the underlying agent used.

Key points to know:

- remote write format does not contain explicit definition of any metric types while metadata definition is still in flux and can have different implementation depending on the remote-write compatible agent
- remote read is a separate interface and it is much less defined. For example, remote read may not work without precise queries; see [here](https://prometheus.io/docs/prometheus/latest/storage/#remote-storage-integrations) and [here](https://github.com/timescale/promscale/issues/64) for details
- some remote-write compatible agents may support additional formats for remote write, like JSON, but it is not part of official Prometheus remote write specification and therefore absent here

### Usage

To build k6 binary with the Prometheus remote write output extension use:
```
xk6 build --with github.com/grafana/xk6-output-prometheus-remote@latest 
```

Then run new k6 binary with:
```
K6_PROMETHEUS_REMOTE_URL=http://localhost:9090/api/v1/write ./k6 run script.js -o output-prometheus-remote
```

Add TLS and HTTP basic authentication:
```
K6_PROMETHEUS_REMOTE_URL=https://localhost:9090/api/v1/write K6_PROMETHEUS_INSECURE_SKIP_TLS_VERIFY=false K6_CA_CERT_FILE=example/tls.crt K6_PROMETHEUS_USER=foo K6_PROMETHEUS_PASSWORD=bar ./k6 run script.js -o output-prometheus-remote
```

Different remote storage agents are supported with mapping option. The default is Prometheus itself but there is a simpler raw mapping that can be used as a starting point for other remote agents:
```
K6_PROMETHEUS_MAPPING=raw K6_PROMETHEUS_REMOTE_URL=http://localhost:9090/api/v1/write ./k6 run script.js -o output-prometheus-remote
```

Note: Prometheus remote client relies on a snappy library for serialization which can panic on [encode operation](https://github.com/golang/snappy/blob/544b4180ac705b7605231d4a4550a1acb22a19fe/encode.go#L22).

### On sample rate

k6 processes its outputs once per second and that is also a default flush period in this extension. The number of k6 builtin metrics is 26 and they are collected at the rate of 50ms. In practice it means that there will be around 1000-1500 samples on average per each flush period in case of raw mapping. If custom metrics are configured, that estimate will have to be adjusted.

Depending on exact setup, it may be necessary to configure Prometheus and / or remote-write agent to handle the load. For example, see [`queue_config` parameter](https://prometheus.io/docs/practices/remote_write/) of Prometheus.

If remote endpoint responds too slowly or the k6 test run generates too many metrics, extension may start discarding samples in order to continue to adhere to the flush period.

### Prometheus as remote-write agent

To enable remote write in Prometheus 2.x use `--enable-feature=remote-write-receiver` option. See docker-compose samples in `example/`. Options for remote write storage can be found [here](https://prometheus.io/docs/operating/integrations/). 


# Docker Compose

This repo includes a [docker-compose.yml](./docker-compose.yml) file that starts _Prometheus_, _Grafana_, and a custom build of _k6_ having the `xk6-output-prometheus-remote` extension.

> This is just a quick setup to show the usage. For a real use case, you will want to deploy outside of docker.

Clone the repo to get started and follow these steps: 

1. Start the docker compose environment.
    ```shell
    docker-compose up -d
    ```
    
    > Some users have encountered failures for the k6 build portion. A workaround may be to disable the _"Use Docker Compose V2"_ checkbox in the _General_ section of Docker Desktop settings.

    ```shell
    # Output
    Creating xk6-output-prometheus-remote_grafana_1     ... done
    Creating xk6-output-prometheus-remote_prometheus_1  ... done
    Creating xk6-output-prometheus-remote_k6_1          ... done
    ```

2. Use the k6 Docker image to run the k6 script and send metrics to the Prometheus container started on the previous step. You must [set the `testid` tag](https://k6.io/docs/using-k6/tags-and-groups/#test-wide-tags) with a unique identifier to segment the metrics into discrete test runs for the Grafana dashboards.
    ```shell
    docker-compose run --rm k6 run -<example/test.js --tag testid=<SOME-ID>
    ```
    For convenience, the `docker-run.sh` can be used to simply:
    ```shell
    ./docker-run.sh example/test.js
    ```

3. Visit http://localhost:3000/ to view results in Grafana.
