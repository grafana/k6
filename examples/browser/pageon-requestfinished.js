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

	// Track all completed requests
	const finishedRequests = []

	page.on('requestfinished', request => {
		finishedRequests.push({
			url: request.url(),
			method: request.method(),
			resourceType: request.resourceType(),
		})

		console.log(`✓ Request finished: ${request.method()} ${request.url()}`)
	})

	await page.goto('https://quickpizza.grafana.com/', { waitUntil: 'networkidle' })

	console.log(`Total requests completed: ${finishedRequests.length}`)

	// Log all API requests
	const apiRequests = finishedRequests.filter(r => r.url.includes('/api/'))
	console.log(`API requests: ${apiRequests.length}`)

	await page.close()
}
