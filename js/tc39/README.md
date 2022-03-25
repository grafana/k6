# Introduction to a k6's TC39 testing

The point of this module is to test k6 goja+babel combo against the tc39 test suite.

Ways to use it:
1. run ./checkout.sh to checkout the last commit sha of [test262](https://github.com/tc39/test262)
   that was tested with this module
2. Run `go test &> out.log`

If there are failures there will be a JSON with what failed.
The full list of failing tests, and the error, is in `breaking_test_errors.json`. All errors list there with the corresponding error will *not* be counted as errors - this is what the test expects, those specific errors.
Due to changes to goja it is not uncommon for the error to change, or there to be now a new error on previously passing test, or (hopefully) a test that was not passing but now is.
In all of those cases `breaking_test_errors.json` needs to be updated. Currently, the output is the *difference* in errors, so we need to "null" the file. To that we set it to an empty JSON `echo '{}' > breaking_test_errors.json`.
Run the test with output to a file `go test &> breaking_test_errors.json`. And then edit `out.log` so only the JSON is left. I personally search for `FAIL` and that should be the first thing just *after* the JSON, delete till the end of file. This is easiest done with sed(or vim) as in `sed -i '/FAIL/,$d' breaking_test_errors.json`.

NOTE: some text editors/IDEs will try to parse files ending in `json` as JSON, which given the size of `breaking_test_errors.json` might be a problem when it's not actually a JSON (before the edit). So it might be a better idea to name it something different if editing by hand and fix it later.

This is a modified version of [the code in the goja
repo](https://github.com/dop251/goja/blob/master/tc39_test.go)


## Reasons for recording breaking_test_errors.json

Unfortunately k6 doesn't pass all the test that are currently defined as "interesting".
Goja decided to just not run the ones that it knows it fails currently, but this
means that if they stop failing, someone needs to go re-enable these tests.

This also means that, if the
previous breakage was something a user can work around in a certain way, it now might be something
else that the user can't workaround or have another problem.

On top of that this more or less exist as k6 add babel to the mix and that changes which tests pass and how some of them fail. When we remove babel, it's very likely we will also remove this package as it won't be relevant anymore.

For these reasons, I decided that recording what breaks and checking that it doesn't change is better.
