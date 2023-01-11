import { chromium } from 'k6/x/browser';

export const options = {
  thresholds: {
    checks: ["rate==1.0"]
  }
}

export default async function() {
  const browser = chromium.launch({
    headless: __ENV.XK6_HEADLESS ? true : false,
  });

  // grant camera and microphone permissions to the
  // new browser context.
  const context = browser.newContext({
    permissions: ["camera", "microphone"],
  });

  const page = context.newPage();

  try {
    await page.goto('http://whatsmyuseragent.org/');
    page.screenshot({ path: `example-chromium.png` });
  } finally {
    page.close();
    browser.close();
  }
}
