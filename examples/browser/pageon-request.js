import { browser } from 'k6/browser'

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
}

export default async function () {
  const page = await browser.newPage()

  // registers a handler that logs all requests made by the page
  page.on('request', async request => console.log(request.url()))

  await page.goto('https://quickpizza.grafana.com/', { waitUntil: 'networkidle' })

  await page.close();
}