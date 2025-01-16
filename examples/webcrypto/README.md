# k6 webcrypto examples

In this directory, you will find examples of how to use the k6's `webcrypto` module in your k6 scripts.

> [!IMPORTANT]
> We do run the tests based on these examples, that's why we have a simple convention for each example:
>
> * It should do any `console.log`. Since we try to detect that output (log) contain `INFO` keyword.
> * It should NOT `try/catch` exceptions. Since we try to detect if keywords like `"Uncaught"` and `"ERRO"` should not appear in the output (logs).

See [`../../js/modules/k6/webcrypto/cmd_run_test.go`](../../js/modules/k6/webcrypto/cmd_run_test.go) for more details.
