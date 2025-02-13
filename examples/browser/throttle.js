import { browser, networkProfiles } from 'k6/browser';

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
      startTime: "10s",
    },
    cpuThrottled: {
      executor: 'shared-iterations',
      options: {
        browser: {
            type: 'chromium',
        },
      },
      exec: 'cpuThrottled',
      startTime: "5s",
    },
  },
  thresholds: {
    'browser_http_req_duration{scenario:normal}': ['p(99)<3000'],
    'browser_http_req_duration{scenario:networkThrottled}': ['p(99)<6000'],
    'iteration_duration{scenario:normal}': ['p(99)<5000'],
    'iteration_duration{scenario:cpuThrottled}': ['p(99)<10000'],
  },
}

export async function normal() {
  const context = await browser.newContext();
  const page = await context.newPage();

  try {
    await page.goto('https://test.k6.io/', { waitUntil: 'networkidle' });
  } finally {
    await page.close();
  }
}

export async function networkThrottled() {
  const context = await browser.newContext();
  const page = await context.newPage();

  try {
    await page.throttleNetwork(networkProfiles["Slow 3G"]);

    await page.goto('https://test.k6.io/', { waitUntil: 'networkidle' });
  } finally {
    await page.close();
  }
}

export async function cpuThrottled() {
  const context = await browser.newContext();
  const page = await context.newPage();

  try {
    await page.throttleCPU({ rate: 4 });

    await page.goto('https://test.k6.io/', { waitUntil: 'networkidle' });
  } finally {
    await page.close();
  }
}
