# Dashboards

Add custom dashboards here, then start the [docker-compose](../../docker-compose.yml) to have the dashboards provisioned at startup. Use folders to group related dashboards.
The docker-compose setup comes with some pre-built Grafana dashboards. One for listing the discrete test runs as a list, one for visualizing the results of a specific test run, and another for Apdex score.

>Note: The dashboards work with the Native Histogram mapping so it is required to enable it.

### Test result dashboard

[<img src="/images/prometheus-dashboard-k6-test-result.png" width="500"/>](/images/prometheus-dashboard-k6-test-result.png)

Results can be filtered by:

- testid
- scenario
- url

[<img src="/images/prometheus-dashboard-k6-test-result-variables.png" width="500"/>](/images/prometheus-dashboard-k6-test-result-variables.png)

Response time metrics are based on the **metrics** variable, and the values can be:

- k6_http_req_duration_seconds (default)
- k6_http_req_waiting_seconds

The board is structured into 4 sections

#### Performance Overview

[<img src="/images/prometheus-dashboard-k6-test-result-performance.png" width="500"/>](/images/prometheus-dashboard-k6-test-result-performance.png)

#### HTTP

[<img src="/images/prometheus-dashboard-k6-test-result-http.png" width="500"/>](/images/prometheus-dashboard-k6-test-result-http.png)

#### Scenarios

[<img src="/images/prometheus-dashboard-k6-test-result-scenarios.png" width="500"/>](/images/prometheus-dashboard-k6-test-result-scenarios.png)

### Test list dashboard

[<img src="/images/prometheus-dashboard-k6-test-runs.png" width="500"/>](/images/prometheus-dashboard-k6-test-runs.png)

>Note: This dashboard depends on the use of testid tag


#### Apdex Overview Dashboard

[<img src="/images/prometheus-dashboard-k6-test-result-apdex.png" width="500"/>](/images/prometheus-dashboard-k6-test-result-apdex.png)

The Apdex score is calculated based on your SLA ```([T]target time (seconds) Apdex
    variable, default 0.3 sec)``` required where you can define a response time threshold of T seconds, where all responses handled in T seconds or less satisfy the end user.

If you want to know more

<https://medium.com/@tristan_96324/prometheus-apdex-alerting-d17a065e39d0>

<https://en.wikipedia.org/wiki/Apdex>

<!-- 

#### Custom Metrics Example Dashboard

[<img src="/images/prometheus-dashboard-k6-test-result-apdex.png" width="500"/>](/images/prometheus-dashboard-k6-test-result-apdex.png)

This dashboard is an example of a dashboard with panels showing custom metrics.

To test this dashboard, you must run the test.

If you want to know more

<https://medium.com/@tristan_96324/prometheus-apdex-alerting-d17a065e39d0>

<https://en.wikipedia.org/wiki/Apdex> -->
