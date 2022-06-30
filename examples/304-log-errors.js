import { check } from 'k6';
import launcher from 'k6/x/browser';

export const options = {
  scenarios: {
    err: {
      executor: 'shared-iterations',
      vus: 1,
      iterations: 20,
      exec: 'locatorStrictMode',
      // exec: 'pageGotoTimeout0',
      // exec: 'pageGotoTimeout100',
      // exec: 'pageQuery',
      // exec: 'pageWaitForSelector',
    }
  }
}

export function locatorStrictMode() {
  const browser = launcher.launch('chromium', {
    headless: __ENV.XK6_HEADLESS ? true : false,
  });
  const context = browser.newContext();
  const page = context.newPage();

  page.goto('https://test.k6.io/', { waitUntil: 'networkidle' });

// ===== BEFORE =====
// ERRO[0002] click: element handle cannot evaluate: cannot call function on expression ("\n\t\t(node, injected, selector, strict, state, timeout, ...args) => {\n\t\t\treturn injected.waitForSelector(selector, node, strict, state, 'raf', timeout, ...args);\n\t\t}\n\t\n//# sourceURL=__xk6_browser_evaluation_script__\n") in execution context (3) in frame (F229F0E745BC94879FA7D9EA9AC1CA63) with session (DEE3FBD05BF682C55E68A2D972EA215F): exception "Uncaught (in promise)" (0:0): error:strictmodeviolation
//         at reflect.methodValueCall (native)
//         at locatorStrictMode (file:///home/ivan/Projects/grafana/xk6-browser/examples/304-log-errors.js:144:15(41))
//         at native  executor=shared-iterations scenario=err source=stacktrace
//
// ===== AFTER =====
// ERRO[0001] click "a": strict mode violation, multiple elements returned for selector query
//         at reflect.methodValueCall (native)
//         at locatorStrictMode (file:///home/ivan/Projects/grafana/xk6-browser/examples/304-log-errors.js:143:15(41))
//         at native  executor=shared-iterations scenario=err source=stacktrace
  page.locator('a').click();

  page.close();
  browser.close();
}

export function pageGotoTimeout0() {
  const browser = launcher.launch('chromium', {
    headless: __ENV.XK6_HEADLESS ? true : false,
  });
  const context = browser.newContext();
  const page = context.newPage();

// ===== BEFORE =====
// ERRO[0001] timed out
//         at reflect.methodValueCall (native)
//         at pageGotoTimeout0 (file:///home/ivan/Projects/grafana/xk6-browser/examples/304-log-errors.js:29:50(32))
//         at native  executor=shared-iterations scenario=err source=stacktrace
//
// ===== AFTER =====
// ERRO[0001] frame navigation to "https://test.k6.io/my_messages.php": timed out after 0s
//         at reflect.methodValueCall (native)
//         at pageGotoTimeout0 (file:///home/ivan/Projects/grafana/xk6-browser/examples/304-log-errors.js:60:50(32))
//         at native  executor=shared-iterations scenario=err source=stacktrace
  page.goto('https://test.k6.io/my_messages.php', { timeout: 0 });

  page.close();
  browser.close();
}

export function pageGotoTimeout100() {
  const browser = launcher.launch('chromium', {
    headless: __ENV.XK6_HEADLESS ? true : false,
  });
  const context = browser.newContext();
  const page = context.newPage();

// ===== BEFORE =====
// ERRO[0001] timed out
//         at reflect.methodValueCall (native)
//         at pageGotoTimeout100 (file:///home/ivan/Projects/grafana/xk6-browser/examples/304-log-errors.js:56:50(32))
//         at native  executor=shared-iterations scenario=err source=stacktrace
//
// ===== AFTER =====
// ERRO[0001] frame navigation to "https://test.k6.io/my_messages.php": timed out after 100ms
//         at reflect.methodValueCall (native)
//         at pageGotoTimeout100 (file:///home/ivan/Projects/grafana/xk6-browser/examples/304-log-errors.js:85:50(32))
//         at native  executor=shared-iterations scenario=err source=stacktrace
  page.goto('https://test.k6.io/my_messages.php', { timeout: 100 });

  page.close();
  browser.close();
}

export function pageQuery() {
  const browser = launcher.launch('chromium', {
    headless: __ENV.XK6_HEADLESS ? true : false,
  });
  const context = browser.newContext();
  const page = context.newPage();

  page.goto('https://test.k6.io/my_messages.php', { waitUntil: 'networkidle' });

  // Enter login credentials and login
  page.$('input[name="login"]').type('admin');
  page.$('input[name="password"]').type('123');
  page.$('input[type="submit"]').click();

// RARE (race condition)
// ===== BEFORE =====
// ERRO[0003] error waiting for selector: element handle cannot evaluate: cannot call function on expression ("\n\t\t(node, injected, selector, strict, state, timeout, ...args) => {\n\t\t\treturn injected.waitForSelector(selector, node, strict, state, 'raf', timeout, ...args);\n\t\t}\n\t\n//# sourceURL=__xk6_browser_evaluation_script__\n") in execution context (3) in frame (BC46B592CD84782A8E5C9C0D8AFADA1A) with session (2178FD2416327DB14E218B78D7D051C9): Cannot find context with specified id (-32000)
//         at reflect.methodValueCall (native)
//         at pageWaitForSelector (file:///home/ivan/Projects/grafana/xk6-browser/examples/304-log-errors.js:132:29(70))
//         at native  executor=shared-iterations scenario=err source=stacktrace
//
// ===== AFTER =====
// ERRO[0008] querying selector "h5": execution context with ID 3 not found
//         at reflect.methodValueCall (native)
//         at pageQuery (file:///home/ivan/Projects/grafana/xk6-browser/examples/304-log-errors.js:125:15(70))
//         at native  executor=shared-iterations scenario=err source=stacktrace
  page.$('h5');

  page.close();
  browser.close();
}

export function pageWaitForSelector() {
  const browser = launcher.launch('chromium', {
    headless: __ENV.XK6_HEADLESS ? true : false,
  });
  const context = browser.newContext();
  const page = context.newPage();

  page.goto('https://test.k6.io/my_messages.php', { waitUntil: 'networkidle' });

  // Enter login credentials and login
  page.$('input[name="login"]').type('admin');
  page.$('input[name="password"]').type('123');
  page.$('input[type="submit"]').click();

// RARE (race condition)
// ===== BEFORE =====
// ERRO[0010] error waiting for selector: element handle cannot evaluate: cannot call function on expression ("\n\t\t(node, injected, selector, strict, state, timeout, ...args) => {\n\t\t\treturn injected.waitForSelector(selector, node, strict, state, 'raf', timeout, ...args);\n\t\t}\n\t\n//# sourceURL=__xk6_browser_evaluation_script__\n") in execution context (3) in frame (845A2D612F368B0F02970E7507330D6E) with session (B41D468207D8A59BCF55EAC5E57083D0): Cannot find context with specified id (-32000)
//         at reflect.methodValueCall (native)
//         at pageWaitForSelector (file:///home/ivan/Projects/grafana/xk6-browser/examples/304-log-errors.js:129:29(70))
//         at native  executor=shared-iterations scenario=err source=stacktrace
//
// ... or ...
//
// ERRO[0012] error waiting for selector: element handle cannot evaluate: cannot call function on expression ("\n\t\t(node, injected, selector, strict, state, timeout, ...args) => {\n\t\t\treturn injected.waitForSelector(selector, node, strict, state, 'raf', timeout, ...args);\n\t\t}\n\t\n//# sourceURL=__xk6_browser_evaluation_script__\n") in execution context (5) in frame (7734FD7138F93BE05501F59C24C43C54) with session (2826C5F95263529C5A702CAD5A96A738): exception "Uncaught (in promise)" (0:0): timed out after 0ms
//         at reflect.methodValueCall (native)
//         at pageWaitForSelector (file:///home/ivan/Projects/grafana/xk6-browser/examples/304-log-errors.js:129:29(70))
//         at native  executor=shared-iterations scenario=err source=stacktrace
//
// ===== AFTER =====
// ERRO[0003] err:websocket: close 1006 (abnormal closure): unexpected EOF  category="Connection:handleIOError" elapsed="0 ms" goroutine=488
// ERRO[0004] waitForSelector "h5": execution context with ID 3 not found
//         at reflect.methodValueCall (native)
//         at pageWaitForSelector (file:///home/ivan/Projects/grafana/xk6-browser/examples/304-log-errors.js:163:29(70))
//         at native  executor=shared-iterations scenario=err source=stacktrace
//
// ... or ...
//
// ERRO[0003] waitForSelector "h5": timed out after 0ms
//         at reflect.methodValueCall (native)
//         at pageWaitForSelector (file:///home/ivan/Projects/grafana/xk6-browser/examples/304-log-errors.js:163:29(70))
//         at native  executor=shared-iterations scenario=err source=stacktrace
  page.waitForSelector('h5', { timeout: 0 });

  page.close();
  browser.close();
}
