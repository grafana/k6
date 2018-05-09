# Information for confluent-kafka-go developers

Whenever librdkafka error codes are updated make sure to run generate before building:

```
  $ (cd go_rdkafka_generr && go install) && go generate
  $ go build
```




## Testing

Some of the tests included in this directory, the benchmark and integration tests in particular,
require an existing Kafka cluster and a testconf.json configuration file to
provide tests with bootstrap brokers, topic name, etc.

The format of testconf.json is a JSON object:
```
{
  "Brokers": "<bootstrap-brokers>",
  "Topic": "<test-topic-name>"
}
```

See testconf-example.json for an example and full set of available options.


To run unit-tests:
```
$ go test
```

To run benchmark tests:
```
$ go test -bench .
```

For the code coverage:
```
$ go test -coverprofile=coverage.out -bench=.
$ go tool cover -func=coverage.out
```

## Build tags (static linking)


Different build types are supported through Go build tags (`-tags ..`),
these tags should be specified on the **application** build command.

 * `static` - Build with librdkafka linked statically (but librdkafka
              dependencies linked dynamically).
 * `static_all` - Build with all libraries linked statically.
 * neither - Build with librdkafka (and its dependencies) linked dynamically.



## Generating HTML documentation

To generate one-page HTML documentation run the mk/doc-gen.py script from the
top-level directory. This script requires the beautifulsoup4 Python package.

```
$ source .../your/virtualenv/bin/activate
$ pip install beautifulsoup4
...
$ mk/doc-gen.py > kafka.html
```
