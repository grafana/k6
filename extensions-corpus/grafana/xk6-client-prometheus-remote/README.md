[![GitHub Release](https://img.shields.io/github/v/release/grafana/xk6-client-prometheus-remote)](https://github.com/grafana/xk6-client-prometheus-remote/releases/)
[![Go Report Card](https://goreportcard.com/badge/github.com/grafana/xk6-client-prometheus-remote)](https://goreportcard.com/report/github.com/grafana/xk6-client-prometheus-remote)
[![GitHub Actions](https://github.com/grafana/xk6-client-prometheus-remote/actions/workflows/validate.yml/badge.svg)](https://github.com/grafana/xk6-client-prometheus-remote/actions/workflows/validate.yml)
![GitHub Downloads](https://img.shields.io/github/downloads/grafana/xk6-client-prometheus-remote/total)

# xk6-client-prometheus-remote

**A k6 extension for testing Prometheus Remote Write endpoints**

This extension adds Prometheus Remote Write testing capabilities to [k6](https://go.k6.io/k6). You can test any service that accepts data via Prometheus remote_write API such as [Cortex](https://github.com/cortexproject/cortex), [Thanos](https://github.com/improbable-eng/thanos), [Prometheus itself](https://prometheus.io/docs/prometheus/latest/feature_flags/#remote-write-receiver) and other services [listed here](https://prometheus.io/docs/operating/integrations/#remote-endpoints-and-storage).

It is implemented using the [xk6](https://k6.io/blog/extending-k6-with-xk6/) system.

> :warning: Not to be confused with [Prometheus remote write **output** extension](https://github.com/grafana/xk6-output-prometheus-remote) which is publishing test-run metrics to Prometheus.

## Usage

```javascript
import { check, sleep } from 'k6';
import remote from 'k6/x/remotewrite';

export let options = {
    vus: 10,
    duration: '10s',
};

const client = new remote.Client({
    url: "<your-remote-write-url>"
});

export default function () {
    let res = client.store([{
        "labels": [
            { "name": "__name__", "value": `my_cool_metric_${__VU}` },
            { "name": "service", "value": "bar" }
        ],
        "samples": [
            { "value": Math.random() * 100, }
        ]
    }]);
    check(res, {
        'is status 2xx': (r) => r.status >= 200 && r.status < 300,
    });
    sleep(1)
}
```

The [examples](https://github.com/grafana/xk6-client-prometheus-remote/blob/master/examples) directory contains examples of how to use the xk6-client-prometheus-remote extension. A k6 binary containing the xk6-client-prometheus-remote extension is required to run the examples.

## More advanced use case

The above example shows how you can generate samples in JS and pass them into the extension, the extension will then submit them to the remote_write API endpoint.
When you want to produce samples at a very high rate the overhead which is involved when passing objects from the JS runtime to the extension can get expensive though,
to optimize this the extension also offers the option for the user to pass it a template based on which samples should be generated and then sent,
by generating the samples inside the extension the overhead of passing objects from the JS runtime to the extension can be avoided.

```javascript
const template = {
    __name__: 'k6_generated_metric_${series_id/4}',    // Name of the series.
    series_id: '${series_id}',                         // Each value of this label will match 1 series.
    cardinality_1e1: '${series_id/10}',                // Each value of this label will match 10 series.
    cardinality_2: '${series_id%2}',                   // Each value of this label will match 2 series.
};

write_client.storeFromTemplates(
    100,                // minimum random value
    200,                // maximum random value
    1643235433 * 1000,  // timestamp in ms
    42,                 // series id range start
    45,                 // series id range end
    template,
);
```

The above code could generate and send the following 3 samples, values are randomly chosen from the defined range:

```
Metric:    k6_generated_metric_10{cardinality_1e1="4", cardinality_2="0", series_id="42"},
Timestamp: 16432354331000
Value:     193
---
Metric:    k6_generated_metric_10{cardinality_1e1="4", cardinality_2="1", series_id="43"},
Timestamp: 16432354331000
Value:     121
---
Metric:    k6_generated_metric_11{cardinality_1e1="4", cardinality_2="0", series_id="44"},
Timestamp: 16432354331000
Value:     142
```

## Download

You can download pre-built k6 binaries from the [Releases](https://github.com/grafana/xk6-client-prometheus-remote/releases/) page.

**Build**

The [xk6](https://github.com/grafana/xk6) build tool can be used to build a k6 that will include xk6-client-prometheus-remote extension:

```bash
$ xk6 build --with github.com/grafana/xk6-client-prometheus-remote@latest
```

For more build options and how to use xk6, check out the [xk6 documentation](https://github.com/grafana/xk6).

## Validation

To validate the extension end-to-end against a real Prometheus instance in Docker, run:

```bash
$ scripts/validate.sh
```

This builds a k6 binary with the extension, starts Prometheus with the remote write receiver enabled, remote-writes a metric, and confirms it is queryable. Useful for catching wire/protocol regressions after a Prometheus dependency bump. Requires `docker`, `xk6`, and `curl`.

## Contribute

If you want to contribute or help with the development of **xk6-client-prometheus-remote**, start by reading [CONTRIBUTING.md](CONTRIBUTING.md). 

## Feedback

If you find the xk6-client-prometheus-remote extension useful, please star the repo. The number of stars will determine the time allocated for maintenance.

[![Stargazers over time](https://starchart.cc/grafana/xk6-client-prometheus-remote.svg?variant=adaptive)](https://starchart.cc/grafana/xk6-client-prometheus-remote)
