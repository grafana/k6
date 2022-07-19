<p align="center"><a href="https://k6.io/"><img src="assets/logo.svg" alt="xk6-browser" width="220" height="220" /></a></p>

<h3 align="center">Browser automation and end-to-end web testing for k6</h3>
<p align="center">An extension for k6 adding browser-level APIs with rough Playwright compatibility.</p>

<p align="center">
  <a href="https://github.com/grafana/xk6-browser/releases"><img src="https://img.shields.io/github/release/grafana/xk6-browser.svg" alt="Github release"></a>
  <a href="https://github.com/grafana/xk6-browser/actions/workflows/all.yaml"><img src="https://github.com/grafana/xk6-browser/actions/workflows/all.yaml/badge.svg" alt="Build status"></a>
  <a href="https://goreportcard.com/report/github.com/grafana/xk6-browser"><img src="https://goreportcard.com/badge/github.com/grafana/xk6-browser" alt="Go Report Card"></a>
  <br>
  <a href="https://twitter.com/k6_io"><img src="https://img.shields.io/badge/twitter-@k6_io-55acee.svg" alt="@k6_io on Twitter"></a>
  <a href="https://k6.io/slack"><img src="https://img.shields.io/badge/Slack-k6-ff69b4.svg" alt="Slack channel"></a>
</p>
<p align="center">
    <a href="https://github.com/grafana/xk6-browser/releases">Download</a> ·
    <a href="#install">Install</a> ·
    <a href="https://k6.io/docs/javascript-api/k6-x-browser/">Documentation</a> ·
    <a href="https://community.k6.io/c/xk6-browser/14">Community Forum</a>
</p>

<br/>
<img src="assets/github-hr.svg" height="32" alt="---" />
<br/>

**xk6-browser** is a [k6](https://k6.io/) extension adding support for automation of browsers via the [Chrome Devtools Protocol](https://chromedevtools.github.io/devtools-protocol/) (CDP).

Special acknowledgment to the authors of [Playwright](https://playwright.dev/) and [Puppeteer](https://github.com/puppeteer/puppeteer) for their trailblazing work in this area. This project is heavily influenced and in some regards based on the code of those projects.

## Goals

- Bring browser automation to the k6 testing platform while supporting core k6 features like VU executors, scenarios, metrics, checks, thresholds, logging, DNS remapping, IP blocklists, etc.
- Test stability as the top priority by supporting non-flaky [selectors](https://playwright.dev/docs/selectors) combined with [auto-waiting](https://playwright.dev/docs/actionability/) for actions just like Playwright.
- Aim for rough API compatibility with [Playwright](https://github.com/microsoft/playwright). The reason for this is two-fold; for one we don't want users to have to learn a completely new API just to use xk6-browser, and secondly, it opens up for using the [Playwright RPC server](https://github.com/mxschmitt/playwright-go) as an optional backend for xk6-browser should we decide to support that in the future.
- Support for Chromium compatible browsers first, and eventually Firefox and WebKit-based browsers.

See our [project roadmap](ROADMAP.md) for more details.


## FAQ

- **Is this production ready?**<br>
    No, not yet. We're focused on making the extension stable and reliable, as that's our top priority, before adding more features.

- **Is this extension supported in k6 Cloud?**<br>
    No, not yet. Once the codebase is deemed production ready we'll add support for browser-based testing in k6 Cloud.

- **It doesn't work with my Chromium/Chrome version, why?**<br>
    CDP evolves and there are differences between different versions of Chromium, sometimes quite subtle. The codebase is continuously tested with the two latest major releases of Google Chrome.

- **Are Firefox or WebKit-based browsers supported?**<br>
    Not yet. There are differences in CDP coverage between Chromium, Firefox, and WebKit-based browsers. xk6-browser is initially only targetting Chromium-based browsers.

- **Are all features of Playwright supported?**<br>
    No. Playwright's API is pretty large and some of the functionality only makes sense if it's implemented using async operations: event listening, request interception, waiting for events, etc. This requires the existence of an event loop per VU in k6, which was only [recently added](https://github.com/grafana/k6/issues/882). Most of the current xk6-browser API is synchronous and thus lacks some of the functionality that requires asynchronicity, but we're gradually migrating existing methods to return a `Promise`, and adding new ones that will follow the same API.

    Expect many breaking changes during this transition, which we'll point out in the release notes.

    Note that `async`/`await` is still not natively supported in k6 scripts, because of the outdated Babel version it uses. If you wish to use this syntax you'll have to transform your script beforehand with an updated Babel version. See the [k6-template-es6 project](https://github.com/grafana/k6-template-es6) and [this comment](https://github.com/grafana/k6/issues/779#issuecomment-964027280) for details.

## Install

### Pre-built binaries

The easiest way to install xk6-browser is to grab a pre-built binary from the [GitHub Releases](https://github.com/grafana/xk6-browser/releases) page. Once you download and unpack the release, you can optionally copy the xk6-browser binary it contains somewhere in your `PATH`, so you are able to run xk6-browser from any location on your system.

Note that you **cannot** use the plain k6 binary released by the k6 project and must run any scripts that import `k6/x/browser` with this separate binary.

### Build from source

To build a `k6` binary with this extension, first ensure you have the prerequisites:

- Make sure that you're running [the latest Go version](https://go.dev/dl/)
- [Go toolchain](https://go101.org/article/go-toolchain.html)
- Git

Then:

1. Install `xk6`:
  ```shell
  go install go.k6.io/xk6/cmd/xk6@latest
  ```

2. Build the binary:
  ```shell
  xk6 build --output xk6-browser --with github.com/grafana/xk6-browser
  ```

  This will create a `xk6-browser` binary file in the current working directory. This file can be used exactly the same as the main `k6` binary, with the addition of being able to run xk6-browser scripts.

3. Run scripts that import `k6/x/browser` with the new `xk6-browser` binary. On Linux and macOS make sure this is done by referencing the file in the current directory:
   ```shell
   ./xk6-browser run <script>
   ```

   Note: You can place it somewhere in your `PATH` so that it can be run from anywhere on your system.

## Examples

#### Launch options

```js
import { chromium } from 'k6/x/browser';

export default function() {
    const browser = chromium.launch({
        args: [],                   // Extra commandline arguments to include when launching browser process
        debug: true,                // Log all CDP messages to k6 logging subsystem
        devtools: true,             // Open up developer tools in the browser by default
        env: {},                    // Environment variables to set before launching browser process
        executablePath: null,       // Override search for browser executable in favor of specified absolute path
        headless: false,            // Show browser UI or not
        ignoreDefaultArgs: [],      // Ignore any of the default arguments included when launching browser process
        proxy: {},                  // Specify to set browser's proxy config
        slowMo: '500ms',            // Slow down input actions and navigations by specified time
        timeout: '30s',             // Default timeout to use for various actions and navigations
    });
    browser.close();
}
```

#### New browser context options

```js
import { chromium } from 'k6/x/browser';

export default function() {
    const browser = chromium.launch();
    const context = browser.newContext({
        acceptDownloads: false,             // Whether to accept downloading of files by default
        bypassCSP: false,                   // Whether to bypass content-security-policy rules
        colorScheme: 'light',               // Preferred color scheme of browser ('light', 'dark' or 'no-preference')
        deviceScaleFactor: 1.0,             // Device scaling factor
        extraHTTPHeaders: {name: "value"},  // HTTP headers to always include in HTTP requests
        geolocation: {latitude: 0.0, longitude: 0.0},       // Geolocation to use
        hasTouch: false,                    // Simulate device with touch or not
        httpCredentials: {username: null, password: null},  // Credentials to use if encountering HTTP authentication
        ignoreHTTPSErrors: false,           // Ignore HTTPS certificate issues
        isMobile: false,                    // Simulate mobile device or not
        javaScriptEnabled: true,            // Should JavaScript be enabled or not
        locale: 'en-US',                    // The locale to set
        offline: false,                     // Whether to put browser in offline mode or not
        permissions: ['midi'],              // Permisions to grant by default
        reducedMotion: 'no-preference',     // Indicate to browser whether it should try to reduce motion/animations
        screen: {width: 800, height: 600},  // Set default screen size
        timezoneID: '',                     // Set default timezone to use
        userAgent: '',                      // Set default user-agent string to use
        viewport: {width: 800, height: 600},// Set default viewport to use
    });
    browser.close();
}
```

#### Page screenshot

```js
import { chromium } from 'k6/x/browser';

export default function() {
    const browser = chromium.launch({ headless: false });
    const context = browser.newContext();
    const page = context.newPage();
    page.goto('http://whatsmyuseragent.org/');
    page.screenshot({ path: `example-chromium.png` });
    page.close();
    browser.close();
}
```

#### Query DOM for element using CSS, XPath or Text based selectors

```js
import { chromium } from 'k6/x/browser';

export default function() {
    const browser = chromium.launch({ headless: false });
    const context = browser.newContext();
    const page = context.newPage();
    page.goto('http://whatsmyuseragent.org/');

    // Find element using CSS selector
    let ip = page.$('.ip-address p').textContent();
    console.log("CSS selector: ", ip);

    // Find element using XPath expression
    ip = page.$("//div[@class='ip-address']/p").textContent();
    console.log("Xpath expression: ", ip);

    // Find element using Text search (TODO: support coming soon!)
    //ip = page.$("My IP Address").textContent();
    //console.log("Text search: ", ip);

    page.close();
    browser.close();
}
```

#### Evaluate JS in browser

```js
import { chromium } from 'k6/x/browser';

export default function() {
    const browser = chromium.launch({ headless: false });
    const context = browser.newContext();
    const page = context.newPage();
    page.goto('http://whatsmyuseragent.org/', { waitUntil: 'load' });
    const dimensions = page.evaluate(() => {
        return {
            width: document.documentElement.clientWidth,
            height: document.documentElement.clientHeight,
            deviceScaleFactor: window.devicePixelRatio
        };
    });
    console.log(JSON.stringify(dimensions));
    page.close();
    browser.close();
}
```

#### Set preferred color scheme of browser

```js
import { chromium } from 'k6/x/browser';
import { sleep } from "k6";

export default function() {
    const browser = chromium.launch({
        headless: false
    });
    const context = browser.newContext({
        colorScheme: 'dark', // Valid values are "light", "dark" or "no-preference"
    });
    const page = context.newPage();
    page.goto('http://whatsmyuseragent.org/');

    sleep(5);

    page.close();
    browser.close();
}
```

#### Fill out a form

```js
import { chromium } from 'k6/x/browser';

export default function() {
    const browser = chromium.launch({
        headless: false,
        slowMo: '500ms' // slow down by 500ms
    });
    const context = browser.newContext();
    const page = context.newPage();

    // Goto front page, find login link and click it
    page.goto('https://test.k6.io/', { waitUntil: 'networkidle' });
    const elem = page.$('a[href="/my_messages.php"]');
    elem.click();

    // Enter login credentials and login
    page.$('input[name="login"]').type('admin');
    page.$('input[name="password"]').type('123');
    page.$('input[type="submit"]').click();

    // Wait for next page to load
    page.waitForLoadState('networkidle');

    page.close();
    browser.close();
}
```

#### Check element state

```js
import { chromium } from 'k6/x/browser';
import { check } from "k6";

export default function() {
    const browser = chromium.launch({
        headless: false
    });
    const context = browser.newContext();
    const page = context.newPage();

    // Inject page content
    page.setContent(`
        <div class="visible">Hello world</div>
        <div style="display:none" class="hidden"></div>
        <div class="editable" editable>Edit me</div>
        <input type="checkbox" enabled class="enabled">
        <input type="checkbox" disabled class="disabled">
        <input type="checkbox" checked class="checked">
        <input type="checkbox" class="unchecked">
    `);

    // Check state
    check(page, {
        'visible': p => p.$('.visible').isVisible(),
        'hidden': p => p.$('.hidden').isHidden(),
        'editable': p => p.$('.editable').isEditable(),
        'enabled': p => p.$('.enabled').isEnabled(),
        'disabled': p => p.$('.disabled').isDisabled(),
        'checked': p => p.$('.checked').isChecked(),
        'unchecked': p => p.$('.unchecked').isChecked() === false,
    });

    page.close();
    browser.close();
}
```

#### Locator API

We suggest using the Locator API instead of the low-level
`ElementHandle` methods. An element handle can go stale if
the element's underlying frame is navigated. However,
with the Locator API, even if the underlying frame
navigates, locators will continue to work.

The Locator API can also help you abstract a page to simplify testing.
To do that, you can use a pattern called the Page Object Model.
You can see an example [here](examples/locator_pom.js).

```js
import { chromium } from 'k6/x/browser';

export default function () {
  const browser = chromium.launch({
    headless: false,
  });
  const context = browser.newContext();
  const page = context.newPage();

  page.goto("https://test.k6.io/flip_coin.php", {
    waitUntil: "networkidle",
  });

  /*
  In this example, we will use two locators, matching a
  different betting button on the page. If you were to query
  the buttons once and save them as below, you would see an
  error after the initial navigation. Try it!

    const heads = page.$("input[value='Bet on heads!']");
    const tails = page.$("input[value='Bet on tails!']");

  The Locator API allows you to get a fresh element handle each
  time you use one of the locator methods. And, you can carry a
  locator across frame navigations. Let's create two locators;
  each locates a button on the page.
  */
  const heads = page.locator("input[value='Bet on heads!']");
  const tails = page.locator("input[value='Bet on tails!']");

  const currentBet = page.locator("//p[starts-with(text(),'Your bet: ')]");

  // the tails locator clicks on the tails button by using the
  // locator's selector.
  tails.click();
  // Since clicking on each button causes page navigation,
  // waitForNavigation is needed. It's because the page
  // won't be ready until the navigation completes.
  page.waitForNavigation();
  console.log(currentBet.innerText());

  // the heads locator clicks on the heads button by using the
  // locator's selector.
  heads.click();
  page.waitForNavigation();
  console.log(currentBet.innerText());

  tails.click();
  page.waitForNavigation();
  console.log(currentBet.innerText());

  page.close();
  browser.close();
}
```

## Status

Currently only Chromium is supported, and the [Playwright API](https://playwright.dev/docs/api/class-playwright) coverage is as follows:

| Class | Support | Missing APIs |
|   :---   | :--- | :--- |
| [Accessibility](https://playwright.dev/docs/api/class-accessibility) | :warning: | [`snapshot()`](https://playwright.dev/docs/api/class-accessibility#accessibilitysnapshotoptions) |
| [Browser](https://playwright.dev/docs/api/class-browser) | :white_check_mark: | [`startTracing()`](https://playwright.dev/docs/api/class-browser#browser-start-tracing), [`stopTracing()`](https://playwright.dev/docs/api/class-browser#browser-stop-tracing) |
| [BrowserContext](https://playwright.dev/docs/api/class-browsercontext) | :white_check_mark: | [`addCookies()`](https://playwright.dev/docs/api/class-browsercontext#browsercontextaddcookiescookies), [`backgroundPages()`](https://playwright.dev/docs/api/class-browsercontext#browser-context-background-pages), [`cookies()`](https://playwright.dev/docs/api/class-browsercontext#browser-context-cookies), [`exposeBinding()`](https://playwright.dev/docs/api/class-browsercontext#browser-context-expose-binding), [`exposeFunction()`](https://playwright.dev/docs/api/class-browsercontext#browser-context-expose-function), [`newCDPSession()`](https://playwright.dev/docs/api/class-browsercontext#browser-context-new-cdp-session), [`on()`](https://playwright.dev/docs/api/class-browsercontext#browser-context-event-background-page), [`route()`](https://playwright.dev/docs/api/class-browsercontext#browser-context-route), [`serviceWorkers()`](https://playwright.dev/docs/api/class-browsercontext#browser-context-service-workers), [`storageState()`](https://playwright.dev/docs/api/class-browsercontext#browser-context-storage-state), [`unroute()`](https://playwright.dev/docs/api/class-browsercontext#browser-context-unroute), [`waitForEvent()`](https://playwright.dev/docs/api/class-browsercontext#browser-context-wait-for-event), [`tracing`](https://playwright.dev/docs/api/class-browsercontext#browser-context-tracing) |
| [BrowserServer](https://playwright.dev/docs/api/class-browserserver) | :warning: | All |
| [BrowserType](https://playwright.dev/docs/api/class-browsertype) | :white_check_mark: | [`connect()`](https://playwright.dev/docs/api/class-browsertype#browser-type-connect), [`connectOverCDP()`](https://playwright.dev/docs/api/class-browsertype#browser-type-connect-over-cdp), [`launchPersistentContext()`](https://playwright.dev/docs/api/class-browsertype#browsertypelaunchpersistentcontextuserdatadir-options), [`launchServer()`](https://playwright.dev/docs/api/class-browsertype#browsertypelaunchserveroptions) |
| [CDPSession](https://playwright.dev/docs/api/class-cdpsession) | :warning: | All |
| [ConsoleMessage](https://playwright.dev/docs/api/class-consolemessage) | :warning: | All |
| [Coverage](https://playwright.dev/docs/api/class-coverage) | :warning: | All |
| [Dialog](https://playwright.dev/docs/api/class-dialog) | :warning: | All |
| [Download](https://playwright.dev/docs/api/class-download) | :warning: | All |
| [ElementHandle](https://playwright.dev/docs/api/class-elementhandle) | :white_check_mark: | [`$eval()`](https://playwright.dev/docs/api/class-elementhandle#element-handle-eval-on-selector), [`$$eval()`](https://playwright.dev/docs/api/class-elementhandle#element-handle-eval-on-selector-all), [`setInputFiles()`](https://playwright.dev/docs/api/class-elementhandle#element-handle-set-input-files) |
| [FetchRequest](https://playwright.dev/docs/api/class-fetchrequest) | :warning: | All |
| [FetchResponse](https://playwright.dev/docs/api/class-fetchresponse) | :warning: | All |
| [FileChooser](https://playwright.dev/docs/api/class-filechooser) | :warning: | All |
| [Frame](https://playwright.dev/docs/api/class-frame) | :white_check_mark: | [`$eval()`](https://playwright.dev/docs/api/class-frame#frame-eval-on-selector), [`$$eval()`](https://playwright.dev/docs/api/class-frame#frame-eval-on-selector-all), [`addScriptTag()`](https://playwright.dev/docs/api/class-frame#frame-add-script-tag), [`addStyleTag()`](https://playwright.dev/docs/api/class-frame#frame-add-style-tag), [`dragAndDrop()`](https://playwright.dev/docs/api/class-frame#frame-drag-and-drop), [`locator()`](https://playwright.dev/docs/api/class-frame#frame-locator), [`setInputFiles()`](https://playwright.dev/docs/api/class-frame#frame-set-input-files) |
| [JSHandle](https://playwright.dev/docs/api/class-jshandle) | :white_check_mark: | - |
| [Keyboard](https://playwright.dev/docs/api/class-keyboard) | :white_check_mark: | - |
| [Locator](https://playwright.dev/docs/api/class-locator) | :white_check_mark: | [`allInnerTexts()`](https://playwright.dev/docs/api/class-locator#locator-all-inner-texts), [`allTextContents()`](https://playwright.dev/docs/api/class-locator#locator-all-text-contents), [`boundingBox([options])`](https://playwright.dev/docs/api/class-locator#locator-bounding-box), [`count()`](https://playwright.dev/docs/api/class-locator#locator-count), [`dragTo(target[, options])`](https://playwright.dev/docs/api/class-locator#locator-drag-to), [`elementHandle([options]) (state: attached)`](https://playwright.dev/docs/api/class-locator#locator-element-handle), [`elementHandles()`](https://playwright.dev/docs/api/class-locator#locator-element-handles), [`evaluate(pageFunction[, arg, options])`](https://playwright.dev/docs/api/class-locator#locator-evaluate), [`evaluateAll(pageFunction[, arg])`](https://playwright.dev/docs/api/class-locator#locator-evaluate-all), [`evaluateHandle(pageFunction[, arg, options])`](https://playwright.dev/docs/api/class-locator#locator-evaluate-handle), [`first()`](https://playwright.dev/docs/api/class-locator#locator-first), [`frameLocator(selector)`](https://playwright.dev/docs/api/class-locator#locator-frame-locator), [`frameLocator(selector)`](https://playwright.dev/docs/api/class-page#page-frame-locator), [`highlight()`](https://playwright.dev/docs/api/class-locator#locator-highlight), [`last()`](https://playwright.dev/docs/api/class-locator#locator-last), [`nth(index)`](https://playwright.dev/docs/api/class-locator#locator-nth), [`page()`](https://playwright.dev/docs/api/class-locator#locator-page), [`screenshot([options])`](https://playwright.dev/docs/api/class-locator#locator-screenshot), [`scrollIntoViewIfNeeded([options])`](https://playwright.dev/docs/api/class-locator#locator-scroll-into-view-if-needed), [`selectText([options])`](https://playwright.dev/docs/api/class-locator#locator-select-text), [`setChecked(checked[, options])`](https://playwright.dev/docs/api/class-locator#locator-set-checked), [`setInputFiles(files[, options])`](https://playwright.dev/docs/api/class-locator#locator-set-input-files) |
| [Logger](https://playwright.dev/docs/api/class-logger) | :warning: | All |
| [Mouse](https://playwright.dev/docs/api/class-mouse) | :white_check_mark: | - |
| [Page](https://playwright.dev/docs/api/class-page) | :white_check_mark: | [`$eval()`](https://playwright.dev/docs/api/class-page#page-eval-on-selector), [`$$eval()`](https://playwright.dev/docs/api/class-page#page-eval-on-selector-all), [`addInitScript()`](https://playwright.dev/docs/api/class-page#page-add-init-script), [`addScriptTag()`](https://playwright.dev/docs/api/class-page#page-add-script-tag), [`addStyleTag()`](https://playwright.dev/docs/api/class-page#page-add-style-tag), [`dragAndDrop()`](https://playwright.dev/docs/api/class-page#page-drag-and-drop), [`exposeBinding()`](https://playwright.dev/docs/api/class-page#page-expose-binding), [`exposeFunction()`](https://playwright.dev/docs/api/class-page#page-expose-function), [`frame()`](https://playwright.dev/docs/api/class-page#page-frame), [`goBack()`](https://playwright.dev/docs/api/class-page#page-go-back), [`goForward()`](https://playwright.dev/docs/api/class-page#page-go-forward), [`on()`](https://playwright.dev/docs/api/class-page#page-event-close), [`pause()`](https://playwright.dev/docs/api/class-page#page-pause), [`pdf()`](https://playwright.dev/docs/api/class-page#page-pdf), [`route()`](https://playwright.dev/docs/api/class-page#page-route), [`unroute()`](https://playwright.dev/docs/api/class-page#page-unroute), [`video()`](https://playwright.dev/docs/api/class-page#page-video), [`waitForEvent()`](https://playwright.dev/docs/api/class-page#page-wait-for-event), [`waitForResponse()`](https://playwright.dev/docs/api/class-page#page-wait-for-response), [`waitForURL()`](https://playwright.dev/docs/api/class-page#page-wait-for-url), [`workers()`](https://playwright.dev/docs/api/class-page#page-workers) |
| [Request](https://playwright.dev/docs/api/class-request) | :white_check_mark: | [`failure()`](https://playwright.dev/docs/api/class-request#request-failure), [`postDataJSON()`](https://playwright.dev/docs/api/class-request#request-post-data-json), [`redirectFrom()`](https://playwright.dev/docs/api/class-request#request-redirected-from), [`redirectTo()`](https://playwright.dev/docs/api/class-request#request-redirected-to) |
| [Response](https://playwright.dev/docs/api/class-response) | :white_check_mark: | [`finished()`](https://playwright.dev/docs/api/class-response#response-finished) |
| [Route](https://playwright.dev/docs/api/class-route) | :warning: | All |
| [Selectors](https://playwright.dev/docs/api/class-selectors) | :warning: | All |
| [Touchscreen](https://playwright.dev/docs/api/class-touchscreen) | :white_check_mark: | - |
| [Tracing](https://playwright.dev/docs/api/class-tracing) | :warning: | All |
| [Video](https://playwright.dev/docs/api/class-video) | :warning: | All |
| [WebSocket](https://playwright.dev/docs/api/class-websocket) | :warning: | All |
| [Worker](https://playwright.dev/docs/api/class-worker) | :warning: | All |
