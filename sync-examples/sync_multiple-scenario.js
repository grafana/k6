import { browser } from 'k6/x/browser';

export const options = {
  scenarios: {
    messages: {
      executor: 'constant-vus',
      exec: 'messages',
      vus: 2,
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
      vus: 2,
      iterations: 4,
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
  const page = browser.newPage();

  try {
    await page.goto('https://test.k6.io/my_messages.php', { waitUntil: 'networkidle' });
  } finally {
    page.close();
  }
}

export async function news() {
  const page = browser.newPage();

  try {
    await page.goto('https://test.k6.io/news.php', { waitUntil: 'networkidle' });
  } finally {
    page.close();
  }
}
