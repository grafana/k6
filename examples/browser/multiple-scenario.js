import { browser } from 'k6/browser';
import { expect } from "https://jslib.k6.io/k6-testing/0.5.0/index.js";

export const options = {
  scenarios: {
    messages: {
      executor: 'constant-vus',
      exec: 'messages',
      vus: 1,
      duration: '2s',
      options: {
        browser: {
            type: 'chromium',
        },
      },
    },
    news: {
      executor: 'per-vu-iterations',
      exec: 'news',
      vus: 1,
      iterations: 2,
      maxDuration: '5s',
      options: {
        browser: {
            type: 'chromium',
        },
      },
    },
  },
  thresholds: {
    browser_web_vital_fcp: ['max < 5000'],
    checks: ["rate==1.0"]
  }
}

export async function messages() {
  const page = await browser.newPage();

  try {
    const response = await page.goto('https://quickpizza.grafana.com/my_messages.php', { waitUntil: 'networkidle' });
    expect(response.status()).toBe(200);
  } finally {
    await page.close();
  }
}

export async function news() {
  const page = await browser.newPage();

  try {
    await page.goto('https://quickpizza.grafana.com/news.php', { waitUntil: 'networkidle' });
  } finally {
    await page.close();
  }
}
