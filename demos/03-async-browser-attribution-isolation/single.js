import { browser } from "k6/browser";

const MIN_SCREENSHOT_BYTES = 1_000_000;

export const options = {
  scenarios: {
    ui: {
      executor: "shared-iterations",
      vus: 1,
      iterations: 1,
      options: {
        browser: {
          type: "chromium",
        },
      },
    },
  },
};

export default async function () {
  const context = await browser.newContext();
  const page = await context.newPage();

  try {
    await page.goto("https://grafana.com/", { waitUntil: "networkidle" });
    const buf = await page.screenshot({ fullPage: true, type: "png" });
    const size = buf.byteLength;
    console.log(`fullPage screenshot bytes=${size}`);
    if (size < MIN_SCREENSHOT_BYTES) {
      throw new Error(
        `expected screenshot size >= ${MIN_SCREENSHOT_BYTES} bytes, got ${size}`
      );
    }
  } finally {
    await page.close();
    await context.close();
  }
}
