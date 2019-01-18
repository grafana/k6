TODO: Intro

## New Features!

### New option: Setting a file for the console.log` to be redirected to (#833)

You can now specify a file for all things logged by `console.log` to get written to. The CLI flag is `--console-output` and the env variable is `K6_CONSOLE_OUTPUT` with no way to be configured from inside the script for security reasons.

Thanks to @cheesedosa for both proposing and implementing this!

## Bugs fixed!

* JS: Consistently report setup/teardown timeouts as such and switch the error message to be more
  expressive (#890)
* JS: Correctly exit with non zero exit code when setup or teardown timeouts (#892)
* Thresholds: When outputting metrics to the Load Impact cloud, fix the incorrect reporting of
  threshold statuses at the end of the test (#894)
