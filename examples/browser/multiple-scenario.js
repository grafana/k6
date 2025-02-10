import { browser } from 'k6/browser';

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
    await page.goto('https://test.k6.io/my_messages.php', { waitUntil: 'networkidle' });
  } finally {
    await page.close();
  }
}

export async function news() {
  const page = await browser.newPage();

  try {
    await page.goto('https://test.k6.io/news.php', { waitUntil: 'networkidle' });
  } finally {
    await page.close();
  }
}
