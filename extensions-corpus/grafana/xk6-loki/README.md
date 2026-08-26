# xk6-loki

**A k6 extension for pushing logs to Loki.**


## Getting started

1. Install `xk6`

   ```bash
   go install go.k6.io/xk6/cmd/xk6@latest
   ```

1. Checkout `grafana/xk6-loki`

   ```bash
   git clone https://github.com/grafana/xk6-loki
   cd xk6-loki
   ```

1. Build the extension

   ```bash
   make k6
   ```

## Javascript API

### Module `k6/x/loki`

The `k6/x/loki` module contains the Loki extension to interact with the Loki API.
To import the module add

```js
import loki from 'k6/x/loki';
```

on top of your test file.

### Class `Config(url, [timeout, ratio, cardinality, labels])`

The class `Config` holds configuration for the Loki client. The constructor
takes the following arguments:

| argument    | type    | description | default |
| ----------- | ------- | ----------- | ------- |
| url         | string  | The full URL to Loki in format `[${scheme}://][${tenant}[:${token}]]@${host}[:${port}]` | - |
| timeout     | integer | Request timeout in milliseconds, e.g. `10000` for 10s | 10000 |
| ratio       | float   | The ratio between JSON and Protobuf encoded batch requests for pushing logs<br>Must be a number between (including) 0 (100% JSON) and 1 (100% Protobuf)<br>This is only relevant for write scenarios.| 0.9 |
| cardinality | object  | The cardinality (unique values) for [labels](#labels), where the object key is the name of the label and the value is the maximum amount of different values it may have. | null |
| labels      | Labels  | A Labels object that contains custom label definitions. | null |

**Example:**

```js
import loki from 'k6/x/loki';
let conf = loki.Config("localhost:3100");
```

### Class `Labels(labels)`

The class `Labels` allows the definition of custom labels that can be used
instead of the built-in labels. An instance of this class can be passed as
fifth argument to the `loki.Config` constructor.

| argument | type   | description | default |
| -------- | ------ | ----------- | ------- |
| labels   | object | An object of type string (label name) to list of strings (possibel label values). | - |

**Example:**

```js
import loki from 'k6/x/loki';
let labels = loki.Labels({
  "format": ["json", "logfmt"], // must contain at least one of the supported log formats
  "cluster": ["prod-us-east-0", "prod-eu-west-1"],
  "namespace": ["dev", "staging", "prod"],
  "container": ["nginx", "app-1", "app-2", "app-3"]
});
```

### Class `Client(config)`

The class `Client` is a Loki client that can read from and write to a Loki instance.
The constructor takes the following arguments:

| argument | type   | description | default |
| -------- | ------ | ----------- | ------- |
| config   | object |  An instance of `Config` which holds the configuration for the client. | - |

**Example:**

```js
import loki from 'k6/x/loki';
let conf = loki.Config("localhost:3100");
let client = loki.Client(conf);
```

#### Method `client.push()`

This function is a shortcut for `client.pushParameterized(5, 800*1024, 1024*1024)`.

#### Method `client.pushParameterized(streams, minSize, maxSize)`

Execute a push request ([POST /loki/api/v1/push](https://grafana.com/docs/loki/latest/api/#post-lokiapiv1push)).

The function `pushParameterized` generates batch of logs and pushes it to the Loki instance.
A batch consists of one or more streams which hold multiple log lines.
A stream is a set of log lines with a unique set of labels.

| argument | type | description | default |
| -------- | ---- | ----------- | ------- |
| streams | integer | The amount of streams the pushed batch should contain. | - |
| minSize | integer | Minimum size in bytes of the raw log lines of the batch. | - |
| maxSize | integer | Maximum size in bytes of the raw log lines of the batch. | - |

`minSize` and `maxSize` define the boundaries for a random value of the actual batch size.

#### Method `client.instantQuery(query, limit)`

This function is a shortcut for `client.instantQueryAt(query, limit, time.Now())` where `time.Now()` is the current nanosecond.

#### Method `client.instantQueryAt(query, limit, instant)`

Execute an instant query ([GET /loki/api/v1/query](https://grafana.com/docs/loki/latest/api/#get-lokiapiv1query)).

| argument | type    | description                           | default |
|----------|---------|---------------------------------------|---------|
| query    | string  | The LogQL query to perform.           | -       |
| limit    | integer | Maxiumum number of entries to return. | -       |
| instant  | integer | Nanosecond at which to execute query. | -       |

#### Method `client.rangeQuery(query, duration, limit)`

This function is a shortcut for `client.rangeQueryAt(query, duration, limit, time.Now())` where `time.Now()` is the current nanosecond.

#### Method `client.rangeQueryAt(query, duration, limit, instant)`

Execute a range query ([GET /loki/api/v1/query_range](https://grafana.com/docs/loki/latest/api/#get-lokiapiv1query_range)).

| argument | type    | description                                            | default |
|----------|---------|--------------------------------------------------------|---------|
| query    | string  | The LogQL query to perform.                            | -       |
| duration | string  | The time span of the range, e.g. `15m`, `1h`, or `7d`. | -       |
| limit    | integer | Maxiumum number of entries to return.                  | -       |
| instant  | integer | Nanosecond at which to execute query.                  | -       |

`duration` defines the range for the query and uses the current timestamp as end and current timestamp - duration as start.

#### Method `client.labelsQuery(duration)`

This function is a shortcut for `client.labelsQueryAt(duration, time.Now())` where `time.Now()` is the current nanosecond.

#### Method `client.labelsQueryAt(duration, instant)`

Execute a labels query ([GET /loki/api/v1/labels](https://grafana.com/docs/loki/latest/api/#get-lokiapiv1labels)).

| argument | type    | description                                                                   | default |
|----------|---------|-------------------------------------------------------------------------------|---------|
| duration | string  | The time span for which labels should be returned, e.g. `15m`, `1h`, or `7d`. | -       |
| instant  | integer | Nanosecond at which to execute query.                                         | -       |

`duration` defines the range for the query and uses the current timestamp as end and current timestamp - duration as start.

#### Method `client.labelValuesQuery(label, duration)`

This function is a shortcut for `client.labelValuesQueryAt(label, duration, time.Now())` where `time.Now()` is the current nanosecond.

#### Method `client.labelValuesQueryAt(label, duration, instant)`

Execute a label values query ([GET /loki/api/v1/label/<name>/values](https://grafana.com/docs/loki/latest/api/#get-lokiapiv1labelnamevalues)).

| argument | type    | description                                                                         | default |
|----------|---------|-------------------------------------------------------------------------------------|---------|
| label    | string  | The label name for which to query the values.                                       | -       |
| duration | string  | The time span for which label values should be returned, e.g. `15m`, `1h`, or `7d`. | -       |
| instant  | integer | Nanosecond at which to execute query.                                               | -       |

`duration` defines the range for the query and uses the current timestamp as end and current timestamp - duration as start.

#### Method `client.seriesQuery(matchers, duration)`

This function is a shortcut for `client.seriesQueryAt(matchers, duration, time.Now())` where `time.Now()` is the current nanosecond.

#### Method `client.seriesQueryAt(matchers, duration, instant)`

Execute a series query ([GET /loki/api/v1/series](https://grafana.com/docs/loki/latest/api/#series)).

| argument | type    | description                                                                                | default |
|----------|---------|--------------------------------------------------------------------------------------------|---------|
| matchers | list    | A list of label matchers used for the query.                                               | -       |
| duration | string  | The time span for which the matching series should be returned, e.g. `15m`, `1h`, or `7d`. | -       |
| instant  | integer | Nanosecond at which to execute query.                                                      | -       |

`duration` defines the range for the query and uses the current timestamp as end and current timestamp - duration as start.

## Labels

`xk6-loki` uses the following built-in label names for generating streams:

| name | values | notes |
| ---- | ------ | ----- |
| instance | fixed: 1 per k6 worker | |
| format | fixed: apache_common, apache_combined, apache_error, rfc3164, rfc5424, json, logfmt | This label defines how the log lines of a stream are formatted. |
| os | fixed: darwin, linux, windows | - |
| namespace | variable | [^1] |
| app | variable | [^1] |
| pod | variable | [^1] |
| language | variable | [^1] |
| word | variable | [^1] |

[^1]: The amount of values can be defined in `cardinality` argument of the client configuration.

The total amount of different streams is defined by the carthesian product of all label values. Keep in mind that high cardinality impacts the performance of the Loki instance.

### Custom labels

Additionally, `xk6-loki` also supports custom labels that can be used instead
of the built-in labels.

See [examples/custom-labels.js](examples/custom-labels.js) for a full example with custom labels.

## Metrics

The extension collects metrics that are printed in the
[end-of-test summary](https://k6.io/docs/results-visualization/end-of-test-summary/) in addition to the built-in metrics.

### Query metrics

These metrics are collected only for instant and range queries.

| name                              | description                                  |
|-----------------------------------|----------------------------------------------|
| `loki_bytes_processed_per_second` | amount of bytes processed by Loki per second |
| `loki_bytes_processed_total`      | total amount of bytes processed by Loki      |
| `loki_lines_processed_per_second` | amount of lines processed by Loki per second |
| `loki_lines_processed_total`      | total amount of lines processed by Loki      |

### Write metrics

| name | description |
| ---- | ----------- |
| `loki_client_uncompressed_bytes` | the quantity of uncompressed log data pushed to Loki, in bytes |
| `loki_client_lines` | the number of log lines pushed to Loki |

## Example

```js
import { check, sleep } from 'k6';
import loki from 'k6/x/loki';

/**
 * URL used for push and query requests
 * Path is automatically appended by the client
 * @constant {string}
 */
const BASE_URL = `http://localhost:3100`;

/**
 * Client timeout for read and write in milliseconds
 * @constant {number}
 */
const timeout = 5000;

/**
 * Ratio between Protobuf and JSON encoded payloads when pushing logs to Loki
 * @constant {number}
 */
const ratio = 0.5;

/**
 * Cardinality for labels
 * @constant {object}
 */
const cardinality = {
  "app": 5,
  "namespace": 5
};

/**
 * Execution options
 */
export const options = {
  vus: 10,
  iterations: 10,
};

/**
 * Create configuration object
 */
const conf = new loki.Config(BASE_URL, timeout, ratio, cardinality);

/**
 * Create Loki client
 */
const client = new loki.Client(conf);

export default () => {
  // Push a batch of 2 streams with a payload size between 500KB and 1MB
  let res = client.pushParameterized(2, 512 * 1024, 1024 * 1024);
  // A successful push request returns HTTP status 204
  check(res, { 'successful write': (res) => res.status == 204 });
  sleep(1);
}
```

```bash
./k6 run examples/simple.js
```

You can find more examples in the [examples/](./examples) folder.
