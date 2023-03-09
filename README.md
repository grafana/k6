# xk6-output-prometheus-remote

[k6](https://github.com/grafana/k6) extension for publishing test-run metrics to Prometheus via Remote Write endpoint.

> :warning: Not to be confused with [Prometheus Remote Write **client** extension](https://github.com/grafana/xk6-client-prometheus-remote) which is for load testing _Prometheus_ itself.

> :bookmark: As of k6 v0.42.0, this extension is available within k6 as an _experimental module_.
> This means that the extension has entered the process of being fully merged into the core of k6 and does not require a special build with xk6 to utilize this feature.
>
> See the [Prometheus remote write guide](https://k6.io/docs/results-output/real-time/prometheus-remote-write/) to utilize this feature.
>

There are many options for remote-write compatible agents, the official list can be found [here](https://prometheus.io/docs/operating/integrations/). The exact details of how metrics will be processed or stored depends on the underlying agent used.

### Usage

To build k6 binary with the Prometheus remote write output extension use:
```
xk6 build --with github.com/grafana/xk6-output-prometheus-remote@latest 
```

Then run new k6 binary with the following command for using the default configuration (e.g. remote write server url set to `http://localhost:9090/api/v1/write`):
```
./k6 run -o xk6-prometheus-rw script.js 
```

Check [the documentation](https://k6.io/docs/results-output/real-time/prometheus-remote-write) for advanced configurations, Docker Compose ready to use example or for using the builtin experimental output.

## Dashboards

[<img src="./images/prometheus-dashboard-k6-test-result.png" width="500"/>](./images/prometheus-dashboard-k6-test-result.png)

Pre-built Grafana dashboards are available. Check the [dashboard guide](https://k6.io/docs/results-output/real-time/prometheus-remote-write/#time-series-visualization) for details.

>Note: The dashboards work with the Native Histogram mapping so it is required to enable it.
