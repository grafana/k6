import exec from 'k6/execution';
import { chromium } from 'k6/x/browser';

export const options = {
  scenarios: {
    ui: {
      executor: 'shared-iterations',
      options: {
        browser: {
            type: 'chromium',
        },
      },
    },
  }
}

export default function() {
  try {
    const browser = chromium.launch();
    browser.close();
  } catch (e) {
    // The test should not fail when launching the browser without
    // options. Try catch is used to report the error to the shell.
    exec.test.abort(e);
  }
}
