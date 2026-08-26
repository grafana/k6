# <img src="https://github.com/mostafa/xk6-kafka/blob/main/assets/xk6-kafka-logo.png" alt="xk6-kafka logo" style="height: 32px; width:32px;"/> xk6-kafka

[![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/mostafa/xk6-kafka/test.yaml?branch=main&logo=github)](https://github.com/mostafa/xk6-kafka/actions) [![Docker Pulls](https://img.shields.io/docker/pulls/mostafamoradian/xk6-kafka?logo=docker)](https://hub.docker.com/r/mostafamoradian/xk6-kafka) [![Coverage Status](https://coveralls.io/repos/github/mostafa/xk6-kafka/badge.svg?branch=main)](https://coveralls.io/github/mostafa/xk6-kafka?branch=main) [![Go Reference](https://pkg.go.dev/badge/github.com/mostafa/xk6-kafka/v2.svg)](https://pkg.go.dev/github.com/mostafa/xk6-kafka/v2)

The xk6-kafka project is a [k6 extension](https://grafana.com/docs/k6/latest/extensions/) that enables k6 users to load test Apache Kafka using a producer and possibly a consumer for debugging.

The real purpose of this extension is to test the system you meticulously designed to use Apache Kafka. So, you can test your consumers, hence your system, by auto-generating messages and sending them to your system via Apache Kafka.

You can send many messages with each connection to Kafka. These messages are arrays of objects containing a key and a value in various serialization formats, passed via configuration objects. Various serialization formats are supported, including strings, JSON, binary, Avro, and JSON Schema. Avro and JSON Schema can either be fetched from Schema Registry or hard-code directly in the script. SASL PLAIN/SCRAM authentication, SASL GSSAPI ([Kerberose](./docs/kerberos.md)), AWS IAM, Azure Entra OAuth, GCP OAuth and message compression are also supported.

For debugging and testing purposes, a consumer is available to make sure you send the correct data to Kafka.

If you want to learn more about the extension, read the [article](https://grafana.com/blog/2021/09/20/load-testing-kafka-producers-and-consumers/) (outdated) explaining how to load test your Kafka producers and consumers using k6 on the k6 blog. You can also watch [this recording](https://www.youtube.com/watch?v=NQ0fyhq1mxo) of the k6 Office Hours about this extension.

## Supported Features

- **v2.0.0 Performance**: Up to ~383,000 msgs/sec (unacked) with 50 VUs using new `Producer`/`Consumer` constructors with `confluentinc/confluent-kafka-go` (**~3.3x faster than the current v1.x.x/main branch**, which reaches ~115,637 msgs/sec on the same `scripts/test_json.js` benchmark and machine)
- Produce/consume messages as [String](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_string.js), [JSON](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_json.js), [ByteArray](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_bytes.js), [Avro](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_avro_with_schema_registry.js), [JSON Schema](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_jsonschema_with_schema_registry.js), and [Protobuf](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_protobuf_with_schema_registry.js) formats
- Support for user-provided [Avro](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_avro_no_schema_registry.js) and JSON Schema key and value schemas in the script
- Authentication with [SASL PLAIN, SCRAM, SSL and AWS IAM](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_sasl_auth.js), plus [Azure Entra OAuth for Event Hub](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_azure_event_hub.js) and [GCP OAuth for GCP Kafka](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_gcp_kafka.js)
- Create, list and delete [topics](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_topics.js)
- Support for loading [Java Keystore (JKS) files](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_tls_with_jks.js)
- Support for loading Avro schemas from [Schema Registry](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_avro_with_schema_registry.js) with gzip compression support
- Support for [byte array](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_bytes.js) for binary data (from binary protocols)
- Support consumption from all partitions with a [group ID](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_consumer_group.js)
- Support Kafka message compression: Gzip, [Snappy](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_json.js), Lz4 & Zstd
- Support for sending messages with [no key](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_avro_no_schema_registry.js)
- Support for k6 [thresholds](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_json.js) on custom Kafka metrics
- Support for [headers](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_json.js) on produced and consumed messages
- Lots of exported metrics, as shown in the list of [emitted metrics](https://github.com/mostafa/xk6-kafka/blob/main/README.md#emitted-metrics)
- TypeScript definitions available in [api-docs/v2/index.d.ts](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts)

> [!WARNING]
> The v2 API is going to take over the v1 API in the future. So, please migrate to the v2 API as soon as possible, as the v1 API will be deprecated soon.

## Quick Start (v2.0.0+)

```javascript
import { Producer, Consumer, AdminClient } from "k6/x/kafka";

const producer = new Producer({ brokers: ["localhost:9092"], topic: "my-topic" });
const consumer = new Consumer({ brokers: ["localhost:9092"], topic: "my-topic", groupId: "my-group" });
const admin = new AdminClient({ brokers: ["localhost:9092"] });

export default function () {
  producer.produce({ messages: [{ key: "key", value: "value" }] });
  const messages = consumer.consume({ maxMessages: 10 });
}
```

Run with: `./k6 run --vus 50 --duration 60s script.js`

For full examples, see [scripts/v2](./scripts/v2/README.md).

## Download Binaries

### The Official Docker Image

The [official Docker image](https://hub.docker.com/r/mostafamoradian/xk6-kafka) is available on Docker Hub. Before running your script, make the script available to the container by mounting a volume (a directory) or passing it via stdin.

```bash
docker run --rm -i mostafamoradian/xk6-kafka:latest run - <scripts/test_json.js
```

### The Official Binaries

The binaries are generated by the build process and published on the [releases page](https://github.com/mostafa/xk6-kafka/releases). Currently, binaries for GNU/Linux, macOS, and Windows are available for both `amd64` (`x86_64`) and `arm64` architectures.

> [!NOTE]
> If you want to see an official build for your machine, please build and test xk6-kafka from [source](https://grafana.com/docs/k6/latest/extensions/run/build-k6-binary-using-go/) and then create an [issue](https://github.com/mostafa/xk6-kafka/issues/new) with details. I'll add the specific binary to the build pipeline and publish them on the next release.

## Accelerated Local Development with Mise

You can optionally install the [Mise Tool](https://mise.jdx.dev/) for an accelerated, easy, out of the box development experience.

Once Mise is installed, setup your 
[CGO Environment](https://github.com/go101/go101/wiki/CGO-Environment-Setup#use-gcc).

Install tools with `mise install`.

Now you are ready to go!

1. Start local development services with `mise run services` (keep this terminal running)
2. Hack with your editor
3. Build with `mise run build`
4. Run unit tests with `mise run test`
5. Test your binary with `./k6 <path to test script>`
6. Format your files with `mise run format`
7. Verify with `mise run verify`
8. Commit, push and open a PR

If your code passes `mise run verify`, it should pass the PR checks 🎉!

## Build from Source

You can build the k6 binary on various platforms, each with its requirements. The following shows how to build k6 binary with this extension on GNU/Linux distributions.

### Prerequisites

You must have the latest Go version installed to build the k6 binary. The latest version should match [k6](https://github.com/grafana/k6#build-from-source) and [xk6](https://github.com/grafana/xk6#requirements). I recommend [gvm](https://github.com/moovweb/gvm) because it eases version management.

- [gvm](https://github.com/moovweb/gvm) for easier installation and management of Go versions on your machine
- [Git](https://git-scm.com/) for cloning the project
- [xk6](https://github.com/grafana/xk6) for building k6 binary with extensions
- CGO enabled builds with a working native toolchain

### Install and build the latest tagged version

Feel free to skip the first two steps if you already have Go installed.

1. Install gvm by following its [installation guide](https://github.com/moovweb/gvm#installing).
2. Install the latest version of Go using gvm. You need Go 1.4 installed for bootstrapping into higher Go versions, as explained [here](https://github.com/moovweb/gvm#a-note-on-compiling-go-15).
3. Install `xk6`:

   ```shell
   go install go.k6.io/xk6/cmd/xk6@latest
   ```

4. Build the binary with CGO enabled:

   ```shell
   CGO_ENABLED=1 xk6 build --with github.com/mostafa/xk6-kafka/v2@latest
   ```

> [!NOTE]
> Go modules require a `/v2` import path for major version 2 and later. Use `github.com/mostafa/xk6-kafka/v2@…` for `v2.x.x` tags and `github.com/mostafa/xk6-kafka@…` only for `v1.x.x` and earlier.

> [!NOTE]
> You can always use the latest version of k6 to build the extension, but the earliest version of k6 that supports extensions via xk6 is v0.32.0. The xk6 is constantly evolving, so some APIs may not be backward compatible.

### Native build requirements

`xk6-kafka` uses `confluent-kafka-go` and therefore requires `CGO_ENABLED=1` for local builds and tests.

- Linux: use a working C toolchain. If your environment cannot use the bundled `librdkafka`, install `pkg-config` and `librdkafka-dev`.
- macOS: install the Xcode Command Line Tools first. If `pkg-config` cannot resolve `librdkafka`, install `pkg-config` and `librdkafka` with Homebrew.
- Windows: use a Go environment with CGO enabled and a working C toolchain on `PATH`. The GitHub Actions baseline only uses Windows for smoke-build coverage; Kafka-backed tests still run on Linux.

### Build for development

If you want to add a feature or make a fix, clone the project and build it using the following commands. The xk6 will force the build to use the local clone instead of fetching the latest version from the repository. This process enables you to update the code and test it locally.

```bash
git clone git@github.com:mostafa/xk6-kafka.git && cd xk6-kafka
CGO_ENABLED=1 xk6 build --with github.com/mostafa/xk6-kafka/v2@latest=.
```

For local validation, run:

```bash
CGO_ENABLED=1 go test ./...
CGO_ENABLED=1 go test ./... -race
```

## Build using docker

The Grafana xk6 also supports [using docker to build a k6 custom binary with extensions](https://grafana.com/docs/k6/latest/extensions/run/build-k6-binary-using-docker/). This is the simplest way to avoid local CGO and native `librdkafka` setup on development machines.

1. Install the latest xk6 docker image.

   ```shell
   docker pull grafana/xk6
   ```

2. Build the custom binary. On Mac, make sure to add the `GOOS=darwin` option.

   ```shell
   docker run --rm -e GOOS=darwin -u "$(id -u):$(id -g)" -v "${PWD}:/xk6" \
       grafana/xk6 build \
       --with github.com/avitalique/xk6-file@latest \
       --with github.com/LeonAdato/xk6-output-statsd@latest \
       --with github.com/mostafa/xk6-kafka/v2@latest
   ```

## Example scripts

There are many examples in the [script](https://github.com/mostafa/xk6-kafka/blob/main/scripts/) directory that show how to use various features of the extension.

## How to Test Your Kafka Setup

You can start testing your setup immediately, but it takes some time to develop the script, so it would be better to test your script against a development environment and then start testing your environment.

### Development environment

I recommend the [fast-data-dev](https://github.com/lensesio/fast-data-dev) Docker image by Lenses.io, a Kafka setup for development that includes Kafka, Zookeeper, Schema Registry, Kafka-Connect, Landoop Tools, 20+ connectors. It is relatively easy to set up if you have Docker installed. Just monitor Docker logs to have a working setup before attempting to test because the initial setup, leader election, and test data ingestion take time.

1. Run the Kafka environment and expose the ports:

   ```bash
   docker run \
       --detach --rm \
       --name lensesio \
       -p 2181:2181 \
       -p 3030:3030 \
       -p 8081-8083:8081-8083 \
       -p 9581-9585:9581-9585 \
       -p 9092:9092 \
       -e ADV_HOST=127.0.0.1 \
       -e RUN_TESTS=0 \
       lensesio/fast-data-dev:latest
   ```

2. After running the command, visit [localhost:3030](http://localhost:3030) to get into the fast-data-dev environment.

3. You can run the command to see the container logs:

   ```bash
   docker logs -f -t lensesio
   ```

> [!NOTE]
> If you have errors running the Kafka development environment, refer to the [fast-data-dev documentation](https://github.com/lensesio/fast-data-dev).

### The xk6-kafka API

All the exported functions are available by importing the module object from `k6/x/kafka`. Versioned declarations and generated references live under [`api-docs/`](https://github.com/mostafa/xk6-kafka/tree/main/api-docs). For the current v2 surface, use [`api-docs/v2/index.d.ts`](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts) and [`api-docs/v2/docs/README.md`](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/docs/README.md). The legacy unversioned snapshot remains available at [`api-docs/docs/README.md`](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/docs/README.md).

> [!NOTE]
> The JavaScript API is stable as of version 1.0.0 and is not subject to major changes in future versions unless a new major version is released.

## v2 API Migration

`v2.0.0` introduces three new JavaScript constructors:

- `Producer` replaces `Writer`
- `Consumer` replaces `Reader`
- `AdminClient` replaces `Connection`

The v1 names are still exported in `v2.x` as deprecated compatibility aliases over the same Confluent-backed runtime path. See [MIGRATION.md](./MIGRATION.md) for the current parity matrix and deprecation notes.

Current compatibility notes:

- `Writer`, `Reader`, and `Connection` continue to work in `v2.x`, but new examples should prefer `Producer`, `Consumer`, and `AdminClient`.
- `ConnectionConfig` now also accepts `brokers` for the new `AdminClient` constructor, while the legacy `Connection` constructor still accepts `address`.
- `consumer.consume({ maxMessages })` is the new spelling; `reader.consume({ limit })` remains supported.
- `Producer`/`Consumer` continue to emit the legacy `kafka_writer_*` and `kafka_reader_*` custom metric names in `v2.0.0` for dashboard and threshold compatibility.
- Custom writer balancer configuration is not supported on the Confluent compatibility path and should be treated as a migration blocker for now.
- The versioned example suites for the new constructors now live under [`scripts/v2`](./scripts/v2/README.md).

### k6 Test Scripts

The example scripts are available as `test_<format/feature>.js` with more code and commented sections in the [scripts](https://github.com/mostafa/xk6-kafka/blob/main/scripts/) directory. Since this project extends the functionality of k6, it has four stages in the [test life cycle](https://grafana.com/docs/k6/latest/using-k6/test-lifecycle/).

For v2.0.0+ examples using the new constructors (`Producer`, `Consumer`, `AdminClient`), see the [v2 script suite](./scripts/v2/README.md) under `scripts/v2/`.

<details>
<summary>Click to expand detailed usage guide with code examples</summary>

1. To use the extension, you need to import it in your script, like any other JS module:

   ```javascript
   // Either import the module object
   import * as kafka from "k6/x/kafka";

   // Or individual classes and constants
   import { sleep } from "k6";
   import {
     Producer,
     Consumer,
     AdminClient,
     SchemaRegistry,
     SCHEMA_TYPE_STRING,
   } from "k6/x/kafka";
   ```

2. You need to instantiate the classes in the `init` context. All the [k6 options](https://k6.io/docs/using-k6/k6-options/) are also configured here:

   ```javascript
   // Creates a new Producer object to produce messages to Kafka
   const producer = new Producer({
     brokers: ["localhost:9092"],
     topic: "my-topic",
   });

   const consumer = new Consumer({
     brokers: ["localhost:9092"],
     topic: "my-topic",
     groupId: "my-group",
   });

   const admin = new AdminClient({
     brokers: ["localhost:9092"],
   });

   const schemaRegistry = new SchemaRegistry({
     url: "http://localhost:8081",
   });

   // Create topic in setup() to avoid race conditions with multiple VUs
   export function setup() {
     // AdminClient must be created inside setup() to ensure proper VU context
     const setupAdmin = new AdminClient({
       brokers: ["localhost:9092"],
     });
     setupAdmin.createTopic({
       // TopicConfig object
       topic: "my-topic",
       numPartitions: 10, // optional, defaults to 1
       replicationFactor: 1, // optional, defaults to 1
     });

     // Verify topic was created
     const topics = setupAdmin.listTopics();
     if (!topics.some((topic) => topic.topic === "my-topic")) {
       throw new Error("Topic was not created successfully");
     }

     setupAdmin.close();

     // Wait for Kafka metadata to propagate to all brokers
     // This ensures Producer/Consumer can see all partitions
     sleep(2);
   }
   ```

   > [!IMPORTANT]
   > Do NOT use `if (__VU == 0)` at module level for topic creation. This causes race conditions where other VUs start before the topic is created. Always use the `setup()` function or set `autoCreateTopic: true` on the Writer/Producer config.
   >
   > **Alternative**: If you don't need to control partition count, use `autoCreateTopic`:

   ```javascript
   const producer = new Producer({
     brokers: ["localhost:9092"],
     topic: "my-topic",
     autoCreateTopic: true, // Let Producer auto-create the topic
   });
   ```

3. In the VU code, you can produce messages to Kafka or consume messages from it:

   ```javascript
   export default function () {
     // Fetch the list of all topics
     const topics = admin.listTopics();
     console.log(topics); // list of topics

     // Produces message to Kafka
     producer.produce({
       // ProduceConfig object
       messages: [
         // Message object(s)
         {
           key: schemaRegistry.serialize({
             data: "my-key",
             schemaType: SCHEMA_TYPE_STRING,
           }),
           value: schemaRegistry.serialize({
             data: "my-value",
             schemaType: SCHEMA_TYPE_STRING,
           }),
         },
       ],
     });

     // Consume messages from Kafka
     let messages = consumer.consume({
       // ConsumeConfig object
       maxMessages: 10,
     });

     // your messages
     console.log(messages);

     // You can use checks to verify the contents,
     // length and other properties of the message(s)

     // To serialize the data back into a string, you should use
     // the deserialize method of the Schema Registry client. You
     // can use it inside a check, as shown in the example scripts.
     let deserializedValue = schemaRegistry.deserialize({
       data: messages[0].value,
       schemaType: SCHEMA_TYPE_STRING,
     });
   }
   ```

4. In the `teardown` function, close all the connections and possibly delete the topic:

   ```javascript
   export function teardown(data) {
     // Delete the topic
     connection.deleteTopic("my-topic");

     // Close all connections
     writer.close();
     reader.close();
     connection.close();
   }
   ```

5. You can now run k6 with the extension using the following command:

   ```bash
   ./k6 run --vus 50 --duration 60s scripts/test_json.js
   ```

6. On the same machine and with the same `scripts/test_json.js` workload, the current `v1.x.x/main` branch reaches `115,637.233495 msg/s`, while `v2.0.0` reaches `383,331.650997 msg/s`, which is about `3.3x` higher throughput.

7. And here's the `v2.0.0` test result output:

   ```bash
               /\      Grafana   /‾‾/
          /\  /  \     |\  __   /  /
         /  \/    \    | |/ /  /   ‾‾\
        /          \   |   (  |  (‾)  |
       / __________ \  |_|\_\  \_____/


        execution: local
           script: scripts/test_json.js
           output: -

        scenarios: (100.00%) 1 scenario, 50 max VUs, 1m30s max duration (incl. graceful stop):
                 * default: 50 looping VUs for 1m0s (gracefulStop: 30s)



      █ THRESHOLDS

        kafka_reader_error_count
        ✓ 'count == 0' count=0

        kafka_writer_error_count
        ✓ 'count == 0' count=0


      █ TOTAL RESULTS 

      checks_total.......: 1073889 17249.924295/s
      checks_succeeded...: 100.00% 1073889 out of 1073889
      checks_failed......: 0.00%   0 out of 1073889

      ✓ 10 messages are received
      ✓ Topic equals to xk6_kafka_json_topic
      ✓ Key contains key/value and is JSON
      ✓ Value contains key/value and is JSON
      ✓ Header equals {'mykey': 'myvalue'}
      ✓ Time is past
      ✓ Partition is zero
      ✓ Offset is gte zero
      ✓ High watermark is gte zero

      CUSTOM
      kafka_reader_dial_count............: 119321   1916.658255/s
      kafka_reader_dial_seconds..........: avg=0s      min=0s      med=0s      max=0s       p(90)=0s      p(95)=0s     
      kafka_reader_error_count...........: 0        0/s
      kafka_reader_fetch_bytes...........: 274 MB   4.4 MB/s
      kafka_reader_fetch_bytes_max.......: 0        min=0             max=0    
      kafka_reader_fetch_bytes_min.......: 0        min=0             max=0    
      kafka_reader_fetch_size............: 1193210  19166.58255/s
      kafka_reader_fetch_wait_max........: 0s       min=0s            max=0s   
      kafka_reader_fetches_count.........: 1312531  21083.240805/s
      kafka_reader_lag...................: 0        min=0             max=0    
      kafka_reader_message_bytes.........: 274 MB   4.4 MB/s
      kafka_reader_message_count.........: 1193210  19166.58255/s
      kafka_reader_offset................: 23779    min=9             max=24819
      kafka_reader_queue_capacity........: 0        min=0             max=0    
      kafka_reader_queue_length..........: 0        min=0             max=0    
      kafka_reader_read_seconds..........: avg=43.03µs min=11.08µs med=18.45µs max=81.67ms  p(90)=41.12µs p(95)=58.08µs
      kafka_reader_rebalance_count.......: 0        0/s
      kafka_reader_timeouts_count........: 0        0/s
      kafka_reader_wait_seconds..........: avg=0s      min=0s      med=0s      max=0s       p(90)=0s      p(95)=0s     
      kafka_writer_acks_required.........: 0        min=0             max=0    
      kafka_writer_async.................: 0.00%    0 out of 11932100
      kafka_writer_attempts_max..........: 0        min=0             max=0    
      kafka_writer_batch_bytes...........: 2.8 GB   44 MB/s
      kafka_writer_batch_max.............: 0        min=0             max=0    
      kafka_writer_batch_queue_seconds...: avg=0s      min=0s      med=0s      max=0s       p(90)=0s      p(95)=0s     
      kafka_writer_batch_seconds.........: avg=2.67µs  min=437ns   med=729ns   max=30.61ms  p(90)=1.6µs   p(95)=2.04µs 
      kafka_writer_batch_size............: 11932100 191665.825499/s
      kafka_writer_batch_timeout.........: 0s       min=0s            max=0s   
      kafka_writer_error_count...........: 0        0/s
      kafka_writer_message_bytes.........: 5.5 GB   89 MB/s
      kafka_writer_message_count.........: 23864200 383331.650997/s
      kafka_writer_read_timeout..........: 0s       min=0s            max=0s   
      kafka_writer_retries_count.........: 0        0/s
      kafka_writer_wait_seconds..........: avg=0s      min=0s      med=0s      max=0s       p(90)=0s      p(95)=0s     
      kafka_writer_write_count...........: 23864200 383331.650997/s
      kafka_writer_write_seconds.........: avg=5.35µs  min=875ns   med=1.45µs  max=61.23ms  p(90)=3.2µs   p(95)=4.08µs 
      kafka_writer_write_timeout.........: 0s       min=0s            max=0s   

      EXECUTION
      iteration_duration.................: avg=25.07ms min=2.49ms  med=14.17ms max=446.15ms p(90)=61.86ms p(95)=85.65ms
      iterations.........................: 119321   1916.658255/s
      vus................................: 50       min=0             max=50   
      vus_max............................: 50       min=50            max=50

       NETWORK
       data_received......................: 0 B      0 B/s
       data_sent..........................: 0 B      0 B/s




   running (1m02.1s), 00/50 VUs, 117079 complete and 0 interrupted iterations
   default ✓ [======================================] 50 VUs  1m0s
   ```

</details>

### Emitted Metrics

<details>
<summary>Click to expand full metrics table</summary>

`v2.0.0` keeps the existing metric names on the Confluent-backed runtime path. See [MIGRATION.md](./MIGRATION.md#metric-compatibility-appendix) for the compatibility appendix, including the current compatibility-derived semantics and the explicit renamed/removed status (`none` in `v2.0.0`).

| Metric                           | Type    | Description                                                             |
| -------------------------------- | ------- | ----------------------------------------------------------------------- |
| kafka_reader_dial_count          | Counter | Total number of times the reader tries to connect.                      |
| kafka_reader_fetches_count       | Counter | Total number of times the reader fetches batches of messages.           |
| kafka_reader_message_count       | Counter | Total number of messages consumed.                                      |
| kafka_reader_message_bytes       | Counter | Total bytes consumed.                                                   |
| kafka_reader_rebalance_count     | Counter | Total number of rebalances of a topic in a consumer group (deprecated). |
| kafka_reader_timeouts_count      | Counter | Total number of timeouts occurred when reading.                         |
| kafka_reader_error_count         | Counter | Total number of errors occurred when reading.                           |
| kafka_reader_dial_seconds        | Trend   | The time it takes to connect to the leader in a Kafka cluster.          |
| kafka_reader_read_seconds        | Trend   | The time it takes to read a batch of message.                           |
| kafka_reader_wait_seconds        | Trend   | Waiting time before read a batch of messages.                           |
| kafka_reader_fetch_size          | Counter | Total messages fetched.                                                 |
| kafka_reader_fetch_bytes         | Counter | Total bytes fetched.                                                    |
| kafka_reader_offset              | Gauge   | Number of messages read after the given offset in a batch.              |
| kafka_reader_lag                 | Gauge   | The lag between the last message offset and the current read offset.    |
| kafka_reader_fetch_bytes_min     | Gauge   | Minimum number of bytes fetched.                                        |
| kafka_reader_fetch_bytes_max     | Gauge   | Maximum number of bytes fetched.                                        |
| kafka_reader_fetch_wait_max      | Gauge   | The maximum time it takes to fetch a batch of messages.                 |
| kafka_reader_queue_length        | Gauge   | The queue length while reading batch of messages.                       |
| kafka_reader_queue_capacity      | Gauge   | The queue capacity while reading batch of messages.                     |
| kafka_writer_write_count         | Counter | Total number of times the writer writes batches of messages.            |
| kafka_writer_message_count       | Counter | Total number of messages produced.                                      |
| kafka_writer_message_bytes       | Counter | Total bytes produced.                                                   |
| kafka_writer_error_count         | Counter | Total number of errors occurred when writing.                           |
| kafka_writer_batch_seconds       | Trend   | The time it takes to write a batch of messages.                         |
| kafka_writer_batch_queue_seconds | Trend   | The time it takes to queue a batch of messages.                         |
| kafka_writer_write_seconds       | Trend   | The time it takes writing messages.                                     |
| kafka_writer_wait_seconds        | Trend   | Waiting time before writing messages.                                   |
| kafka_writer_retries_count       | Counter | Total number of attempts at writing messages.                           |
| kafka_writer_batch_size          | Counter | Total batch size.                                                       |
| kafka_writer_batch_bytes         | Counter | Total number of bytes in a batch of messages.                           |
| kafka_writer_attempts_max        | Gauge   | Maximum number of attempts at writing messages.                         |
| kafka_writer_batch_max           | Gauge   | Maximum batch size.                                                     |
| kafka_writer_batch_timeout       | Gauge   | Batch timeout.                                                          |
| kafka_writer_read_timeout        | Gauge   | Batch read timeout.                                                     |
| kafka_writer_write_timeout       | Gauge   | Batch write timeout.                                                    |
| kafka_writer_acks_required       | Gauge   | Required Acks.                                                          |
| kafka_writer_async               | Rate    | Async writer.                                                           |

</details>

### FAQ

<details>
<summary>Click to expand FAQ (16 questions)</summary>

1. Why do I receive `Error writing messages`?

    There are a few reasons why this might happen. The most prominent one is that the topic might not exist, which causes the producer to fail to send messages to a non-existent topic.

    **Solution 1 (Recommended)**: Create the topic in the `setup()` function to avoid race conditions:

    ```javascript
    export function setup() {
      const connection = new Connection({ address: "localhost:9092" });
      connection.createTopic({ topic: "my-topic", numPartitions: 10 });

      // Verify and wait for metadata propagation
      const topics = connection.listTopics();
      if (!topics.includes("my-topic")) {
        throw new Error("Topic creation failed");
      }
      connection.close();
      sleep(2); // Allow metadata to propagate
    }
    ```

    **Solution 2**: Set `autoCreateTopic: true` in `WriterConfig` (uses broker defaults):

    ```javascript
    const writer = new Writer({
      brokers: ["localhost:9092"],
      topic: "my-topic",
      autoCreateTopic: true,
    });
    ```

    **Solution 3**: Create a topic manually using the `kafka-topics` command:

    ```bash
    $ docker exec -it lensesio bash
    (inside container)$ kafka-topics --create --topic xk6_kafka_avro_topic --bootstrap-server localhost:9092
    (inside container)$ kafka-topics --create --topic xk6_kafka_json_topic --bootstrap-server localhost:9092
    ```

2. Why does the `reader.consume` keep hanging?

    If the `reader.consume` keeps hanging, it might be because the topic doesn't exist or is empty.

3. I want to test SASL authentication. How should I do that?

    If you want to test SASL authentication, look at [this commit message](https://github.com/mostafa/xk6-kafka/pull/3/commits/403fbc48d13683d836b8033eeeefa48bf2f25c6e), in which I describe how to run a test environment to test SASL authentication.

    For Azure Event Hub with Azure Entra OAuth, use [`scripts/test_azure_event_hub.js`](./scripts/test_azure_event_hub.js) as a reference.

    For GCP Kafka with GCP OAuth, use [`scripts/test_gcp_kafka.js`](./scripts/test_gcp_kafka.js) as a reference.

4. Why doesn't the consumer group consume messages from the topic?

    As explained in issue [#37](https://github.com/mostafa/xk6-kafka/issues/37), multiple inits by k6 cause multiple consumer group instances to be created in the init context, which sometimes causes the random partitions to be selected by each instance. This, in turn, causes confusion when consuming messages from different partitions. This can be solved by using a UUID when naming the consumer group, thereby guaranteeing that the consumer group object was assigned to all partitions in a topic.

5. Why do I receive a `MessageTooLargeError` when I produce messages bigger than 1 MB?

    Kafka has a [maximum message size](https://docs.confluent.io/platform/current/installation/configuration/broker-configs.html) of 1 MB by default, which is set by `message.max.bytes`, and this limit is also applied to the `Writer` object.

    There are two ways to produce larger messages: 1) Change the default value of your Kafka instance to a larger number. 2) Use compression.

    Remember that the `Writer` object will reject messages larger than the default Kafka message size limit (1 MB). Hence you need to set `batchBytes` to a larger value, for example, `1024 * 1024 * 2` (2 MB). The `batchBytes` refers to the raw uncompressed size of all the keys and values (data) in your array of messages you pass to the `Writer` object. You can calculate the raw data size of your messages using [this example script](https://github.com/mostafa/xk6-kafka/issues/181#issuecomment-1325390880).

6. Can I consume messages from a consumer group in a topic with multiple partitions?

    Yes, you can. Just pass the `groupId` to your `Reader` object. You must not specify the partition anymore. (`groupID` remains accepted as a legacy alias.) Visit this [documentation article](https://docs.confluent.io/platform/current/clients/consumer.html#concepts) to learn more about Kafka consumer groups.

    Remember that you must set `sessionTimeout` on your `Reader` object if the consume function terminates abruptly, thus failing to consume messages.

7. Why does the `Reader.consume` produces an `unable to read message` error?

    The `maxWait` option controls how long the reader waits for messages before timing out. If not specified, it uses the default from the underlying Kafka library (typically 1 second). For performance testing reasons, you may want to set a shorter timeout (e.g., 200ms) to avoid hanging. If you keep receiving timeout errors, consider increasing `maxWait` to a larger value:

    ```javascript
    const reader = new Reader({
      brokers: ["localhost:9092"],
      topic: "my-topic",
      maxWait: "5s", // Wait up to 5 seconds for messages
    });
    ```

8. How can I consume from multiple partitions on a single topic?

    You can configure your reader to consume from a (list of) topic(s) and its partitions using a consumer group. This can be achieved by setting `groupTopics`, `groupId` and a few other options for timeouts, intervals and lags. (`groupID` remains accepted as a legacy alias.) Have a look at the [`test_consumer_group.js`](https://github.com/mostafa/xk6-kafka/blob/main/scripts/test_consumer_group.js) example script.

9. How can I use autocompletion in IDEs?

    Copy [`api-docs/v2/index.d.ts`](https://github.com/mostafa/xk6-kafka/blob/main/api-docs/v2/index.d.ts) into your project directory and reference it at the top of your JavaScript file:

    ```javascript
    /// <reference path="index.d.ts" />

    ...
    ```

10. Why timeouts give up sooner than expected?

    There are many ways to configure timeout for the `Reader` and `Writer` objects. They follow Go's time conventions, which means that one second is equal to 1000000000 (one billion). For ease of use, I added the constants that can be imported from the module.

    ```javascript
    import { SECOND } from "k6/x/kafka";

    console.log(2 * SECOND); // 2000000000
    console.log(typeof SECOND); // number
    ```

11. Can I catch errors returned by the consume function?

    Yes. You can catch errors by using a try-catch block. The consume function returns an error object. If the consume function raises, the error object will be populated with the error message.

    ```javascript
    try {
      let messages = reader.consume({
        limit: 10,
      });
    } catch (error) {
      console.error(error);
    }
    ```

12. I am using a nested Avro schema and getting unknown errors. How can I debug them?

    If you have a nested Avro schema and you want to test it against your data, I created a small tool for it, called [nested-avro-schema](https://github.com/mostafa/nested-avro-schema). This tool will help you to find discrepancies and errors in your schema data, so that you can fix them before you run [xk6-kafka](https://github.com/mostafa/xk6-kafka) tests. Refer to [this comment](https://github.com/mostafa/xk6-kafka/issues/266) for more information.

13. What is the difference between hard-coded schemas in the script and the ones fetched from the Schema Registry?

    Read [this comment](https://github.com/mostafa/xk6-kafka/issues/298#issuecomment-2165246467).

14. I want to specify the offset of a message when consuming from a topic. How can I do that?

    To specify the offset of a message while consuming from a topic, use the following options based on your consumption setup:
    1. **When consuming from a group:**
        Use the `startOffset` option in the `Reader` object. This option allows you to define the starting point for message consumption. Here are the values you can use for `startOffset`:
        - `-1`: Consume from the most recent message. This is equivalent to `START_OFFSETS_LAST_OFFSET`.
        - `-2`: Consume from the oldest message. This is equivalent to `START_OFFSETS_FIRST_OFFSET`.
        - Any positive number: Consume from the specific offset number provided.

          The constants `START_OFFSETS_LAST_OFFSET` and `START_OFFSETS_FIRST_OFFSET` are part of the xk6-kafka module. You can import and use them in your script. The `startOffset` option is a string.

          ```javascript
          import { Reader, START_OFFSETS_LAST_OFFSET } from "k6/x/kafka";

          const reader = new Reader({
            brokers: ["localhost:9092"], // Replace with your broker(s)
            groupId: "example-group", // Specify your consumer group ID
            groupTopics: ["example-topic"], // List of topics for the group
            startOffset: START_OFFSETS_LAST_OFFSET, // Use the most recent offset
          });
          ```

    2. **When consuming from a topic:**

        Use the `offset` option instead of `startOffset`. The `offset` option is a number that directly specifies the offset of the message you want to consume, unlike `startOffset`, which is a string.

        ```javascript
        import { Reader } from "k6/x/kafka";

        const reader = new Reader({
          brokers: ["localhost:9092"], // Replace with your broker(s)
          topic: "example-topic", // Specify the topic
          offset: 10, // Consume from offset 10
        });
        ```

15. How can I use Avro union types in my Avro schema?

    Read [this comment](https://github.com/mostafa/xk6-kafka/issues/220#issuecomment-1564061967).

16. What if I want to use a custom profile for the SASL authentication with AWS IAM instead of the default profile?

    You can use the `AWS_PROFILE` environment variable to specify the profile name or use the `awsProfile` option in the `SASLConfig` [object](api-docs/v2/docs/interfaces/SASLConfig.md).

</details>

## Avro Union Types

xk6-kafka uses `hamba/avro` for Avro serialization/deserialization. When working with Avro union types, you can usually provide union values directly without wrapping them in type-specific objects. For nullable fields, you can use `null` directly. For logical primitive unions (for example `int` with `logicalType: "date"`), direct values and wrapped values like `{ "int": 20474 }` or `{ "int.date": 20474 }` are supported and normalized before encoding. See the [Schema Registry documentation](./docs/schema-registry.md#complex-schemas--manage-union-types) for detailed examples and best practices.

## Contributions, Issues and Feedback

I'd be thrilled to receive contributions and feedback on this project. You're always welcome to create an issue if you find one (or many). I would do my best to address the issues. Also, feel free to contribute by opening a PR with changes, and I'll do my best to review and merge it as soon as I can.

## The Release Process

The `main` branch is the _development_ branch, and pull requests are _squashed and merged_ into the `main` branch. When a commit is tagged with a version (e.g., `v1.2.0`), the build pipeline builds the `main` branch at that commit, creating binaries and Docker images. To test the latest unreleased features, clone the `main` branch and build using the local repository as explained in the [build for development](https://github.com/mostafa/xk6-kafka/blob/main/README.md#build-for-development) section.

### Image Signatures

Docker images are signed with [cosign](https://github.com/sigstore/cosign) using keyless signing. You can verify the signature of any image using:

```bash
cosign verify --certificate-identity-regexp ".*" --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  mostafamoradian/xk6-kafka:<version>
```

Replace `<version>` with the specific version tag you want to verify (e.g., `1.2.0`).

## The CycloneDX SBOM

CycloneDX SBOMs in JSON format are generated for `go.mod` and Docker images for each release. They are available in the release assets for each tagged version.

## Disclaimer

This project _was_ a proof of concept but is now used by various companies. It is not officially supported by the k6 team, but rather maintained by [me](https://github.com/mostafa) personally. The JavaScript API is stable as of version 1.0.0, but breaking changes may occur in future major versions.

This project was AGPL3-licensed up until 7 October 2021, and then we [relicensed](https://github.com/mostafa/xk6-kafka/pull/25) it under the [Apache License 2.0](https://github.com/mostafa/xk6-kafka/blob/master/LICENSE).
