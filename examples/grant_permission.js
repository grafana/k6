import { chromium } from 'k6/x/browser';

export const options = {
  thresholds: {
    checks: ["rate==1.0"]
  }
}

export default function() {
  const browser = chromium.launch({
    headless: __ENV.XK6_HEADLESS ? true : false,
  });

  // grant camera and microphone permissions to the
  // new browser context.
  const context = browser.newContext({
    permissions: ["camera", "microphone"],
  });

  const page = context.newPage();
  page.goto('http://whatsmyuseragent.org/').then(() => {
    page.screenshot({ path: `example-chromium.png` });

    page.close();
    browser.close();
  })
}
