TODO: Intro

## New Features!

### New option: Setting a file for the console.log` to be redirected to (#833)

You can now specify a file for all things logged by `console.log` to get written to. The CLI flag is `--console-output` and the env variable is `K6_CONSOLE_OUTPUT` with no way to be configured from inside the script for security reasons.

Thanks to @cheesedosa for both proposing and implementing this!

### New result outputs: StatsD and Datadog (#915)

You can now output any metrics k6 collects to StatsD or Datadog by running `k6 run --out statsd script.js` or `k6 run --out datadog script.js` respectively. Both are very similar, but Datadog has a concept of metric tags, the key-value metadata pairs that will allow you to distinguish between requests for different URLs, response statuses, different groups, etc.

Some details:
- By default both outputs send metrics to a local agent listening on `localhost:8125` (currently only UDP is supported as a transport). You can change this address via the `K6_DATADOG_ADDR` or `K6_STATSD_ADDR` environment variables, by setting their values in the format of `address:port`.
- The new outputs also support adding a `namespace` - a prefix before all the metric names. You can set it via the `K6_DATADOG_NAMESPACE` or `K6_STATSD_NAMESPACE` environment variables respectively. Its default value is `k6.` - notice the dot at the end.
- You can configure how often data batches are sent via the  `K6_STATSD_PUSH_INTERVAL` / `K6_DATADOG_PUSH_INTEVAL` environment variables. The default value is `1s`.
- Another performance tweak can be done by changing the default buffer size of 20 through `K6_STATSD_BUFFER_SIZE` / `K6_DATADOG_BUFFER_SIZE`.
- In the case of Datadog, there is an additional configuration `K6_DATADOG_TAG_BLACKLIST`, which by default is equal to `` (nothing). This is a comma separated list of tags that should *NOT* be sent to Datadog. All other metric tags that k6 emits will be sent.

Thanks to @ivoreis for their work on this!


## Bugs fixed!

* JS: Consistently report setup/teardown timeouts as such and switch the error message to be more
  expressive (#890)
* JS: Correctly exit with non zero exit code when setup or teardown timeouts (#892)
* Thresholds: When outputting metrics to the Load Impact cloud, fix the incorrect reporting of
  threshold statuses at the end of the test (#894)
