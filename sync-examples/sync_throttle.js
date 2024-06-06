import { browser, networkProfiles } from 'k6/x/browser';

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
    networkThrottled: {
      executor: 'shared-iterations',
      options: {
        browser: {
            type: 'chromium',
        },
      },
      exec: 'networkThrottled',
    },
    cpuThrottled: {
      executor: 'shared-iterations',
      options: {
        browser: {
            type: 'chromium',
        },
      },
      exec: 'cpuThrottled',
    },
  },
  thresholds: {
    'browser_http_req_duration{scenario:normal}': ['p(99)<500'],
    'browser_http_req_duration{scenario:networkThrottled}': ['p(99)<3000'],
    'iteration_duration{scenario:normal}': ['p(99)<4000'],
    'iteration_duration{scenario:cpuThrottled}': ['p(99)<10000'],
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

export async function networkThrottled() {
  const context = browser.newContext();
  const page = context.newPage();

  try {
    page.throttleNetwork(networkProfiles['Slow 3G']);

    await page.goto('https://test.k6.io/', { waitUntil: 'networkidle' });
  } finally {
    page.close();
  }
}

export async function cpuThrottled() {
  const context = browser.newContext();
  const page = context.newPage();

  try {
    page.throttleCPU({ rate: 4 });

    await page.goto('https://test.k6.io/', { waitUntil: 'networkidle' });
  } finally {
    page.close();
  }
}
