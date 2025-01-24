# k6 webcrypto examples

In this directory, you will find examples of how to use the k6's `webcrypto` module in your k6 scripts.

> [!IMPORTANT]
> We do run the tests based on these examples; that's why we have a simple convention for each example:
>
> * Success condition: Example SHOULD output an `INFO` keyword using `console.log` at least once.
> * Failure condition: Example SHOULD NOT `try/catch` exceptions so that we can detect failures. The log output SHOULD NOT contain any keywords like `"Uncaught"` and `"ERRO"`.

See [`../../internal/js/modules/k6/webcrypto/cmd_run_test.go`](../../internal/js/modules/k6/webcrypto/cmd_run_test.go) for more details.
