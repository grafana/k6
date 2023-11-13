import { browser } from 'k6/x/browser';

export const options = {
  scenarios: {
    normal: {
      executor: 'shared-iterations',
      options: {
        browser: {
            type: 'chromium',
        },
      },
      exec: 'normal',
    },
    throttled: {
      executor: 'shared-iterations',
      options: {
        browser: {
            type: 'chromium',
        },
      },
      exec: 'throttled',
    },
  },
  thresholds: {
    'browser_http_req_duration{scenario:normal}': ['p(99)<500'],
    'browser_http_req_duration{scenario:throttled}': ['p(99)<1500'],
  },
}

export async function normal() {
  const context = browser.newContext();
  const page = context.newPage();

  try {
    await page.goto('https://test.k6.io/', { waitUntil: 'networkidle' });
  } finally {
    page.close();
  }
}

export async function throttled() {
  const context = browser.newContext();
  const page = context.newPage();

  try {
    page.throttleNetwork({
      latency: 750,
      download: 250,
      upload: 250,
    });

    await page.goto('https://test.k6.io/', { waitUntil: 'networkidle' });
  } finally {
    page.close();
  }
}
