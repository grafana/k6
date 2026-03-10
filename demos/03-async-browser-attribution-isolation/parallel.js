import { browser } from "k6/browser";
import { sleep } from "k6";

export const options = {
  scenarios: {
    screenshot: {
      executor: "shared-iterations",
      exec: "screenshotScenario",
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

export async function screenshotScenario() {
  const context = await browser.newContext();
  const page = await context.newPage();
  try {
    await page.goto("https://grafana.com/");
	await page.waitForLoadState("networkidle");

    const buf = await page.screenshot({ fullPage: true, type: "png" });
    console.log("screenshot-bytes=" + buf.byteLength);
  } finally {
    await page.close();
    await context.close();
  }
}

