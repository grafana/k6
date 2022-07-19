import { check } from 'k6';
import { chromium } from 'k6/x/browser';

export default function() {
  const browser = chromium.launch({
    headless: __ENV.XK6_HEADLESS ? true : false,
  });

  check(browser, {
    'should be connected after launch': browser.isConnected(),
  });

  const handlerCalled = Symbol();

  let p = browser.on('disconnected')
    // The promise resolve/success handler
    .then((val) => {
      check(browser, {
        'should be disconnected on event': !browser.isConnected(),
      });
      return handlerCalled;
    // The promise reject/failure handler
    }, (val) => {
      console.error(`promise rejected: ${val}`);
    });

  p.then((val) => {
    check(val, {
      'the browser.on success handler should be called': val === handlerCalled,
    });
  });

  check(browser, {
    'should be connected before ending iteration': browser.isConnected(),
  });

  // Disconnect from the browser instance.
  browser.close();
}
