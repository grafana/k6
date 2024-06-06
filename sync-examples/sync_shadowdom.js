import { check } from 'k6';
import { browser } from 'k6/x/browser';

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
  },
  thresholds: {
    checks: ["rate==1.0"]
  }
}

export default async function() {
  const page = browser.newPage();
  page.setContent("<html><head><style></style></head><body>hello!</body></html>")
  await page.evaluate(() => {
    const shadowRoot = document.createElement('div');
    shadowRoot.id = 'shadow-root';
    shadowRoot.attachShadow({mode: 'open'});
    shadowRoot.shadowRoot.innerHTML = '<p id="find">Shadow DOM</p>';
    document.body.appendChild(shadowRoot);
  });
  const shadowEl = page.locator("#find");
  check(shadowEl, {
    "shadow element exists": (e) => e !== null,
    "shadow element text is correct": (e) => e.innerText() === "Shadow DOM",
  });
  page.close();
}
