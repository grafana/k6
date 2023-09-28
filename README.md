# xk6-output-prometheus-remote

[k6](https://github.com/grafana/k6) extension for publishing test-run metrics to Prometheus via Remote Write endpoint.

> :bookmark: As of k6 v0.42.0, this extension is available within k6 as an _experimental module_.
> This means that the extension has entered the process of being fully merged into the core of k6 and does not require a special build with xk6 to utilize this feature.
>
> See the [Prometheus remote write guide](https://k6.io/docs/results-output/real-time/prometheus-remote-write/) to utilize this feature.
>

There are many options for remote-write compatible agents, the official list can be found [here](https://prometheus.io/docs/operating/integrations/). The exact details of how metrics will be processed or stored depends on the underlying agent used.

> :warning: Not to be confused with [Prometheus Remote Write **client** extension](https://github.com/grafana/xk6-client-prometheus-remote) which is for load testing _Prometheus_ itself.

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

<p>&nbsp;</p>

[<img src="./images/dashboard-k6-prometheus-upper-section.png" width="500"/>](./images/dashboard-k6-prometheus.png)

This repo contains the [source code](./grafana/dashboards) of two Grafana dashboards designed to visualize test results: [`k6 Prometheus`](https://grafana.com/grafana/dashboards/19665-k6-prometheus/) and [k6 Prometheus (Native Histograms)](https://grafana.com/grafana/dashboards/18030-k6-prometheus-native-histograms/). 

Refer to the [documentation](https://k6.io/docs/results-output/real-time/prometheus-remote-write/#time-series-visualization) to learn more about these dashboards. You can import them to your Grafana instance or with the docker-compose example on this repo. 

🌟 Special thanks to [jwcastillo](https://github.com/jwcastillo) for his contributions and dedication to improving the dashboards. 
