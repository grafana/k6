import { browser } from 'k6/browser'
import { check } from 'https://jslib.k6.io/k6-utils/1.5.0/index.js';

export const options = {
  scenarios: {
    ui: {
      executor: 'shared-iterations',
      options: {
        browser: {
          type: 'chromium',
        },
      },
    },
  },
  thresholds: {
    checks: ["rate==1.0"]
  }
}

export default async function() {
  const context = await browser.newContext()
  const page = await context.newPage()

  try {
    await page.goto('https://quickpizza.grafana.com/browser.php')
    
    const no = await page.locator('#numbers-options')
    await no.tap({ position: { x: 10.65, y: 65.39 } })
    
    await check(page.locator('#select-multiple-info-display'), {
      'displays correctly tapped value': async lo => {
        return await lo.textContent().trim() === 'Selected: three'
      }
    })
  } finally {
    await page.close()
  }
}
