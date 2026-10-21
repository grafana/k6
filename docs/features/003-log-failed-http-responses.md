# Log failed HTTP responses

## Problem

When a load test produces unexpected HTTP responses, failed checks or transport errors, users cannot easily see the response headers and body that explain the failure. The only built-in option, `--http-debug`, logs every request and response, which floods the output under load and hides the few failures that matter. Capturing failures from the script requires repetitive code next to every request and cannot cover transport errors.

## Significance

Investigating failures is a core part of performance testing. The details that identify the root cause, such as request or trace IDs in response headers and error payloads in the body, are exactly what a user needs to hand to the team that owns the failing service.

This affects most k6 users who test real systems. Failures that only appear under high load are the hardest to reproduce, so the first test run is often the only chance to capture them. Users have asked for this repeatedly, for example in #2163 (open since 2021) and #6579.

## Cost of inaction

Users keep "flying blind" when tests fail: they rerun tests with `--http-debug=full` and search through very large logs, or they write and maintain their own logging code in every script. Some move this debugging to other tools, which weakens k6 as the place where performance problems are diagnosed.

## Desired state

A user can enable a single, opt-in setting and get one log entry for each failed HTTP response, containing the information needed to diagnose it: method, URL, status, error, response headers, response body, the VU and iteration, and the reason it was considered a failure.

- "Failed" follows the rules k6 already uses for the `http_req_failed` metric, so `http.expectedStatuses` and custom response callbacks are respected. Transport errors and failed checks on HTTP responses are covered too.
- Successful responses produce no extra output, so the setting is safe to use in large tests.
- Entries go through k6's normal logging, so existing log options and destinations (files, Loki, JSON format) work without extra setup, both locally and in distributed or cloud runs.
- Logged bodies are size-limited to keep the output manageable.

## Out of scope

- Writing failed responses to files from test scripts.
- Attaching response headers or bodies to metrics or metric outputs.
- Protocols other than HTTP (gRPC, WebSockets, browser).
- Logging the request body of failed requests.
