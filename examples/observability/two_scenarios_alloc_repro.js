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
    small: {
      executor: "shared-iterations",
      exec: "smallAllocScenario",
      vus: 1,
      iterations: 1,
    },
  },
};

export async function screenshotScenario() {
  const context = await browser.newContext();
  const page = await context.newPage();
  try {
    await page.goto("https://grafana.com/", { waitUntil: "networkidle" });
    const buf = await page.screenshot({ fullPage: true, type: "png" });
    console.log("screenshot-bytes=" + buf.byteLength);
  } finally {
    await page.close();
    await context.close();
  }
}

export function smallAllocScenario() {
  let s = "";
  while (s.length < 20000) {
    s += "x";
  }
  sleep(1);
  return s.length;
}
