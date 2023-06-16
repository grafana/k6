<p align="center"><a href="https://k6.io/"><img src="assets/k6-logo-with-grafana.svg" alt="k6" width="258" height="210" /></a></p>

<h3 align="center">Like unit testing, for performance</h3>
<p align="center">Modern load testing for developers and testers in the DevOps era.</p>

<p align="center">
  <a href="https://github.com/grafana/k6/releases"><img src="https://img.shields.io/github/release/grafana/k6.svg" alt="Github release"></a>
  <a href="https://github.com/grafana/k6/actions/workflows/all.yml"><img src="https://github.com/grafana/k6/actions/workflows/build.yml/badge.svg" alt="Build status"></a>
  <a href="https://goreportcard.com/report/github.com/grafana/k6"><img src="https://goreportcard.com/badge/github.com/grafana/k6" alt="Go Report Card"></a>
 <a href="https://codecov.io/gh/grafana/k6"><img src="https://img.shields.io/codecov/c/github/grafana/k6/master.svg" alt="Codecov branch"></a>
  <br>
  <a href="https://twitter.com/k6_io"><img src="https://img.shields.io/badge/twitter-@k6_io-55acee.svg" alt="@k6_io on Twitter"></a>
  <a href="https://k6.io/slack"><img src="https://img.shields.io/badge/Slack-k6-ff69b4.svg" alt="Slack channel"></a>
</p>
<p align="center">
    <a href="https://github.com/grafana/k6/releases">Download</a> ·
    <a href="https://k6.io/docs">Documentation</a> ·
    <a href="https://community.k6.io/">Community Forum</a>
</p>

<br/>
<img src="assets/github-hr.png" alt="---" />
<br/>

**k6** is a modern load-testing tool, built on [our years of experience](https://k6.io/about) in the performance and testing industries.
It's built to be powerful, extensible, and full-featured. The key design goal is to provide **the best developer experience**.

Its core features are:

- **Configurable load generation.** Even lower-end machines can simulate lots of traffic.
- **Tests as code.** Reuse scripts, modularize logic, version control, and integrate tests with your CI.
- **A full-featured API.** The scripting API is packed with features that help you simulate real application traffic.
- **An embedded JavaScript engine.** The performance of Go, the scripting familiarity of JavaScript.
- **Multiple Protocol support**. HTTP, WebSockets, gRPC, and more.
- **Large extension ecosystem.** You can extend k6 to support your needs. And many people have already shared their extensions with the community!
- **Flexible metrics storage and visualization**. Summary statistics or granular metrics, exported to the service of your choice.

This is what load testing looks like in the 21st century.

## Example script


```js
import http from "k6/http";
import { check, sleep } from "k6";

// Test configuration
export const options = {
  thresholds: {
    // Assert that 99% of requests finish within 3000ms.
    http_req_duration: ["p(99) < 3000"],
  },
  // Ramp the number of virtual users up and down
  stages: [
    { duration: "30s", target: 15 },
    { duration: "1m", target: 15 },
    { duration: "20s", target: 0 },
  ],
};

// Simulated user behavior
export default function () {
  let res = http.get("https://test-api.k6.io/public/crocodiles/1/");
  // Validate response status
  check(res, { "status was 200": (r) => r.status == 200 });
  sleep(1);
}
```

You can run scripts like this on the CLI, or in your CI, or across a Kubernetes cluster.

## Documentation

The docs cover all aspects of using k6. Some highlights include:
- [Get Started](https://k6.io/docs). Install, run a test, inspect results.
- [HTTP requests](https://k6.io/docs/using-k6/http-requests/). Have your virtual users use HTTP methods.
  Or, check the other [Protocols](https://k6.io/docs/using-k6/protocols/).
- [Thresholds](https://k6.io/docs/using-k6/thresholds). Set goals for your test, and codify your SLOs.
- [Options](https://k6.io/docs/using-k6/k6-options). Configure your load, duration, TLS certificates, and much, much more.
- [Scenarios](https://k6.io/docs/using-k6/scenarios).
  Choose how to model your workload: open models, closed models, constant RPS, fixed iterations, and more.
- [Results output](https://k6.io/docs/results-output). Study, filter, and export your test results.
- [JavaScript API](https://k6.io/docs/javascript-api). Reference and examples of all k6 modules.
- [Extensions](https://k6.io/docs/extensions). Extend k6 for new protocols and use cases.

These links barely scratch the surface! If you're looking for conceptual information, you can read about [Test types](https://k6.io/docs/test-types/introduction/), [Test strategies](https://k6.io/docs/testing-guides/), or one of the many informative [Blog posts](https://k6.io/blog).

## Roadmap

Our team is dedicated to continuously improving and providing the best user experience possible. The [public roadmap](https://github.com/orgs/grafana/projects/443/views/1) covers user-oriented features, UX improvements and JavaScript support that our team will focus on. Remember that timeframes and priorities may shift, but we believe it's important to share our vision.

We hope it provides a clear overview of our plans for future development. We welcome feedback, corrections, and suggestions via GitHub to make it more comprehensive, accessible, and valuable for the community.

## Contribute

If you want to contribute or help with the development of k6, start by reading [CONTRIBUTING.md](CONTRIBUTING.md). Before you start coding, it might be a good idea to first discuss your plans and implementation details with the k6 maintainers—especially when it comes to big changes and features. You can do this in the [GitHub issue](https://github.com/grafana/k6/issues) for the problem you're solving (create one if it doesn't exist).

> **Note:** To disclose security issues, refer to [SECURITY.md](SECURITY.md).

## Support

To get help, report bugs, suggest features, and discuss k6 with others, refer to [SUPPORT.md](SUPPORT.md).

## License

k6 is distributed under the [AGPL-3.0 license](https://github.com/grafana/k6/blob/master/LICENSE.md).

