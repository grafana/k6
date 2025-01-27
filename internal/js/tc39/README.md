# Introduction to a k6's TC39 testing

`js/tc39` package tests k6 [Sobek](https://github.com/grafana/sobek) and the k6 Sobek+esbuild combo against the tc39 test suite.

Ways to use it:
1. run ./checkout.sh to checkout the last commit sha of [test262](https://github.com/tc39/test262)
   that was tested with this module
2. Run `go test &> out.log`

The full list of failing tests, and the error, is in `breaking_test_errors-*.json`. All errors list there with the corresponding error will *not* be counted as errors - this is what the test expects, those specific errors. See reasons for this at the end of this document.

This is a modified version of [the code in the original goja
repo](https://github.com/dop251/goja/blob/master/tc39_test.go) that Sobek was forked from.

## Maintaining the tests

There are few things to keep in mind when maintaining the tests.

* For most of the time, we aim to have the same version (commit hash) of the test suite as the one that [uses Sobek](https://github.com/grafana/sobek/blob/main/.tc39_test262_checkout.sh#L3). So if Sobek brings a new version of the test suite, we should update the version of the test suite in this package as well.
* The new version of Sobek could bring a new functionality, so you might need to re-evaluate the `featuresBlockList`, `skipList` and any other list in the `tc39_test.go` file. The way to go is also keeping this lists closer to the Sobek, however they are not the same, so you might need to adjust them.
* Due to changes to Sobek it's common for the error to change, or there to be now a new error on the previously passing test, or (hopefully) a test that was not passing but now is.
In all of those cases `breaking_test_errors-*.json` needs to be updated. Run the test with `-update` flag to update: `go test -update`.
* Important to mention that there should be a balance between just updating the list of the errors and actually fixing errors. So it's recommended to check the diff of the `breaking_test_errors-*.json` and case by case decide if the error should be updated or the test should be fixed.


## Reasons for recording breaking_test_errors.json

Unfortunately k6 doesn't pass all the test that are currently defined as "interesting".
Goja decided to just not run the ones that it knows it fails currently, but this
means that if they stop failing, someone needs to go re-enable these tests.

This also means that, if the
previous breakage was something a user can work around in a certain way, it now might be something
else that the user can't workaround or have another problem.

Starting v0.53 k6 doesn't use Babel anymore, it now serves tests for `esbuild`, and for the parts uncovered by Sobek's test suite. It is still possible that we drop the whole package in the future.

For these reasons, I decided that recording what breaks and checking that it doesn't change is better.
