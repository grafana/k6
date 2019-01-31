TODO: Intro

## New Features!

### New option: Setting a file for the console.log` to be redirected to (#833)

You can now specify a file for all things logged by `console.log` to get written to. The CLI flag is `--console-output` and the env variable is `K6_CONSOLE_OUTPUT` with no way to be configured from inside the script for security reasons.

Thanks to @cheesedosa for both proposing and implementing this!

### New result outputs: statsd and DataDog (#915)

Both are very similar but DataDog has a concept of tags. By default both send on `localhost:8125` and currently only UDP is supported as transport.
In order to change this you can use the `K6_DATADOG_ADDR` or `K6_STATSD_ADDR` env variable which has to be in the format of `address:port`.
The new outputs also can add a `namespace` which a prefix before all the samples with `K6_DATADOG_NAMESPACE` or `K6_STATSD_NAMESPACE` respectively. By default the value is `k6.`, notice the dot at the end.
In the case of DataDog there is an additional configuration `K6_DATADOG_TAG_WHITELIST` which by default is equal to `status, method`. This is a comma separated list of tags that should be sent to DataDog. All other tags that k6 emits are discarded. This is done because DataDog does indexing on top of the tags and some highly variable tags like `vu` and `iter` will lead to problems with the service.

Thanks to @ivoreis for their work on this!


## Bugs fixed!

* JS: Consistently report setup/teardown timeouts as such and switch the error message to be more
  expressive (#890)
* JS: Correctly exit with non zero exit code when setup or teardown timeouts (#892)
* Thresholds: When outputting metrics to the Load Impact cloud, fix the incorrect reporting of
  threshold statuses at the end of the test (#894)
