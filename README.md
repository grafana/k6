# xk6-browser

A k6 extension adding support for automation of browsers via the Chrome Devtools Protocol (CDP).

Major acknowledgement to the authors of [Playwright](https://playwright.dev/) and [Puppeteer](https://github.com/puppeteer/puppeteer) for their trailblazing work in this area. This project is heavily influenced and in some regards based on the code from those projects.

## Goals

- Bring browser automation to the k6 testing platform while supporting core k6 features like VU executors, scenarios, metrics, checks, thresholds, logging, DNS remapping, IP block lists etc.
- Test stability as top priority by supporting non-flaky [selectors](https://playwright.dev/docs/selectors) combined with [auto-wait](https://playwright.dev/docs/actionability/) for actions just like Playwright
- Aim for rough compatibility with [Playwright](https://github.com/microsoft/playwright). The reason for this is two-fold; for one we don't want users to have to learn a completley new API just to use xk6-browser, and secondly it opens up for using the [Playwright RPC server](https://github.com/mxschmitt/playwright-go) as an optional backend for xk6-browser should we decided to support that.
- Support for Chromium, and eventually Firefox and WebKit based browsers.

## Status

Currently only Chromium is supported, and the [Playwright API](https://playwright.dev/docs/api/class-playwright) coverage is as follows:

|          | Support | Missing APIs |
|   :---   | :--- | :--- |
| [Accessibility](https://playwright.dev/docs/api/class-accessibility) | :warning: | [`snapshot()`](https://playwright.dev/docs/api/class-accessibility#accessibilitysnapshotoptions) | 
| [Browser](https://playwright.dev/docs/api/class-browser) | :white_check_mark: | - |
| [BrowserContext](https://playwright.dev/docs/api/class-browsercontext) | :white_check_mark: Partial | [`addCookies()`](https://playwright.dev/docs/api/class-browsercontext#browsercontextaddcookiescookies) |
| [BrowserServer](https://playwright.dev/docs/api/class-browserserver) | :warning: | All |
| [BrowserType](https://playwright.dev/docs/api/class-browsertype) | :white_check_mark: | [`launchPersistentContext()`](https://playwright.dev/docs/api/class-browsertype#browsertypelaunchpersistentcontextuserdatadir-options), [`launchServer()`](https://playwright.dev/docs/api/class-browsertype#browsertypelaunchserveroptions) |
| [CDPSession](https://playwright.dev/docs/api/class-cdpsession) | :warning: | All |
| [ChromiumBrowser](https://playwright.dev/docs/api/class-chromiumbrowser) | :warning: | TODO |
| [ConsoleMessage](https://playwright.dev/docs/api/class-consolemessage) | :warning: | All |
| [Dialog](https://playwright.dev/docs/api/class-dialog) | :warning: | All |
| [Download](https://playwright.dev/docs/api/class-download) | :warning: | All |
| [ElementHandle](https://playwright.dev/docs/api/class-elementhandle) | :white_check_mark: | - |
| [FileChooser](https://playwright.dev/docs/api/class-filechooser) | :warning: | All |
| [FirefoxBrowser](https://playwright.dev/docs/api/class-firefoxbrowser) | :warning: | All |
| [Frame](https://playwright.dev/docs/api/class-frame) | :white_check_mark: | TODO |
| [JSHandle](https://playwright.dev/docs/api/class-jshandle) | :white_check_mark: | - |
| [Keyboard](https://playwright.dev/docs/api/class-keyboard) | :white_check_mark: | - |
| [Logger](https://playwright.dev/docs/api/class-logger) | :warning: | All |
| [Mouse](https://playwright.dev/docs/api/class-mouse) | :white_check_mark: | - |
| [Page](https://playwright.dev/docs/api/class-page) | :white_check_mark: Partial | TODO |
| [Request](https://playwright.dev/docs/api/class-request) | :white_check_mark: Partial | TODO |
| [Response](https://playwright.dev/docs/api/class-response) | :white_check_mark: Partial | TODO |
| [Route](https://playwright.dev/docs/api/class-route) | :warning: | All |
| [Selectors](https://playwright.dev/docs/api/class-selectors) | :warning: | All |
| [TimeoutError](https://playwright.dev/docs/api/class-timeouterror) | :warning: | All |
| [Touchscreen](https://playwright.dev/docs/api/class-touchscreen) | :white_check_mark: | - |
| [Video](https://playwright.dev/docs/api/class-video) | :warning: | All |
| [WebKitBrowser](https://playwright.dev/docs/api/class-webkitbrowser) | :warning: | All |
| [WebSocket](https://playwright.dev/docs/api/class-websocket) | :warning: | All |
| [Worker](https://playwright.dev/docs/api/class-worker) | :warning: | All |

## Usage

To build a `k6` binary with this extension, first ensure you have the prerequisites:

- [Go toolchain](https://go101.org/article/go-toolchain.html)
- Git

Then:

1. Install `xk6`:
  ```shell
  go install github.com/k6io/xk6/cmd/xk6@latest
  ```

2. Build the binary:
  ```shell
  xk6 build --with github.com/k6io/xk6-browser
  ```

## Examples

#### Page screenshot

```js
import launcher from "k6/x/browser";

export default function() {
    const browser = launcher.launch('chromium', { headless: false });
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
import launcher from "k6/x/browser";

export default function() {
    const browser = launcher.launch('chromium', { headless: false });
    const context = browser.newContext();
    const page = context.newPage();
    page.goto('http://whatsmyuseragent.org/');

    // Find element using CSS selector
    const ip = page.$('.ip-address p').textContent();
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
import launcher from "k6/x/browser";

export default function() {
    const browser = launcher.launch('chromium', { headless: false });
    const context = browser.newContext();
    const page = context.newPage();
    page.goto('http://whatsmyuseragent.org/', { waitUntil: 'networkidle' });
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
import launcher from "k6/x/browser";
import { sleep } from "k6";

export default function() {
    const browser = launcher.launch('chromium', {
        colorScheme: 'dark', // Valid values are "light", "dark" or "no-preference"
        headless: false
    });
    const context = browser.newContext();
    const page = context.newPage();
    page.goto('http://whatsmyuseragent.org/');

    sleep(5);

	page.close();
    browser.close();
}
```

#### Fill out a form

```js
import launcher from "k6/x/browser";

export default function() {
    const browser = launcher.launch('chromium', {
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
    page.waitForLoadState('networkdidle');

	page.close();
    browser.close();
}
```

#### Check element state

```js
import launcher from "k6/x/browser";
import { check } from "k6";

export default function() {
    const browser = launcher.launch('chromium', {
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
