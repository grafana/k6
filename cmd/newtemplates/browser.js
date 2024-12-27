import { browser } from "k6/browser";
import http from "k6/http";
import { sleep, check } from 'k6';

const BASE_URL = __ENV.BASE_URL || "https://quickpizza.grafana.com";

export const options = {
  scenarios: {
    ui: {
      executor: "shared-iterations",
      options: {
        browser: {
          type: "chromium",
        },
      },
    },
  }, {{ if .EnableCloud }}
  cloud: { {{ if .ProjectID }}
    projectID: {{ .ProjectID }}, {{ else }}
    // projectID: 12345, // Replace this with your own projectID {{ end }}
    name: "{{ .ScriptName }}",
  }, {{ end }}
};

export function setup() {
  let res = http.get(BASE_URL);
  if (res.status !== 200) {
    throw new Error(
      `Got unexpected status code ${res.status} when trying to setup. Exiting.`
    );
  }
}

export default async function() {
  let checkData;
  const page = await browser.newPage();
  try {
    await page.goto(BASE_URL);
    checkData = await page.locator("h1").textContent();
    check(page, {
      header: checkData === "Looking to break out of your pizza routine?",
    });
    await page.locator('//button[. = "Pizza, Please!"]').click();
    await page.waitForTimeout(500);
    await page.screenshot({ path: "screenshot.png" });
    checkData = await page.locator("div#recommendations").textContent();
    check(page, {
      recommendation: checkData !== "",
    });
  } finally {
    await page.close();
  }
  sleep(1);
}
