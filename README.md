<h3 align="center">Browser automation and end-to-end web testing for k6</h3>
<p align="center">A module for k6 adding browser-level APIs with rough Playwright compatibility.</p>

<p align="center">
  <a href="https://github.com/grafana/xk6-browser/releases"><img src="https://img.shields.io/github/release/grafana/xk6-browser.svg" alt="Github release"></a>
  <a href="https://github.com/grafana/xk6-browser/actions/workflows/test.yml"><img src="https://github.com/grafana/xk6-browser/actions/workflows/test.yml/badge.svg" alt="Build status"></a>
  <a href="https://goreportcard.com/report/github.com/grafana/xk6-browser"><img src="https://goreportcard.com/badge/github.com/grafana/xk6-browser" alt="Go Report Card"></a>
  <br>
  <a href="https://twitter.com/k6_io"><img src="https://img.shields.io/badge/twitter-@k6_io-55acee.svg" alt="@k6_io on Twitter"></a>
</p>
<p align="center">
    <a href="https://github.com/grafana/k6/releases/latest">Download</a> ·
    <a href="https://grafana.com/docs/k6/latest/javascript-api/k6-browser">Documentation</a> ·
    <a href="https://community.grafana.com/c/grafana-k6/k6-browser/79">Community Forum</a>
</p>

<br/>
<img src="assets/github-hr.svg" height="32" alt="---" />
<br/>

**k6-browser** is a [k6](https://grafana.com/docs/k6/latest/) module that supports browser automation through the [Chrome Devtools Protocol](https://chromedevtools.github.io/devtools-protocol/) (CDP). It adds browser-level APIs to interact with browsers and collect frontend performance metrics as part of your k6 tests.

**As of [k6 version 0.52.0](https://github.com/grafana/k6/releases/tag/v0.52.0), k6-browser is now bundled in k6 as a core module and usable without a separate binary or compilation step!**

Special acknowledgment to the authors of [Playwright](https://playwright.dev/) and [Puppeteer](https://github.com/puppeteer/puppeteer) for their trailblazing work in this area.

## Goals

- Bring browser automation to the k6 testing platform while supporting core k6 features like VU executors, scenarios, metrics, checks, thresholds, logging, DNS remapping, IP blocklists, etc.
- Test stability as the top priority by supporting non-flaky [selectors](https://playwright.dev/docs/api/class-selectors) combined with [auto-waiting](https://playwright.dev/docs/actionability/) for actions just like Playwright.
- Aim for rough API compatibility with [Playwright](https://github.com/microsoft/playwright). The reason for this is twofold: first, we don't want users to have to learn a completely new API just to use xk6-browser, and second, it opens up the possibility of using the [Playwright RPC server](https://github.com/mxschmitt/playwright-go) as an optional backend for the k6 browser module should we decide to support that in the future.
- Support for Chromium-compatible browsers first, and eventually Firefox and WebKit-based browsers.

Please take a look at our [project roadmap](ROADMAP.md) for more details.

## Get started

- [Getting Started](https://grafana.com/docs/k6/latest/using-k6-browser/)
- [Running xk6-browser](https://grafana.com/docs/k6/latest/using-k6-browser/running-browser-tests/)
- [Browser Metrics](https://grafana.com/docs/k6/latest/using-k6-browser/metrics/)
- [Selecting Elements](https://grafana.com/docs/k6/latest/using-k6-browser/recommended-practices/selecting-elements/)

## Documentation

- [Releases](https://github.com/grafana/xk6-browser/releases)
- [API Documentation](https://grafana.com/docs/k6/latest/javascript-api/k6-browser) - This documents how to use our API and its current state with xk6-browser.

## Get help

- If you're having issues installing or running xk6-browser, refer to [TROUBLESHOOTING](/TROUBLESHOOTING.md).
- To get help with usage, report bugs, suggest features, and discuss xk6-browser with other users, see [SUPPORT.md](SUPPORT.md).
- Community discussion happens at the [xk6-browser forum](https://community.grafana.com/c/grafana-k6/k6-browser/79).

## Additional resources

To find out more about xk6-browser or browser testing in general, check out the following links.

**Note:** Since our API is still transitioning, some of the code snippets from the links below might be inaccurate, though the concepts should still be correct. For the latest, please take a look at our up-to-date [API documentation](https://grafana.com/docs/k6/latest/javascript-api/k6-browser).

- [How to add browser-level APIs for web performance metrics](https://grafana.com/blog/2023/01/08/how-to-add-browser-level-apis-for-web-performance-metrics/) - This post explains how to get started with k6 browser to interact with browsers and collect web performance metrics as part of your Grafana k6 tests.
- [Using k6 browser](https://grafana.com/docs/k6/latest/using-k6-browser/) - Get started with the k6 browser module.
