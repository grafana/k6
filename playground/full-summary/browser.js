import {browser} from 'k6/browser'

export async function browserTest() {
	const page = await browser.newPage()

	try {
		await page.goto('https://test.k6.io/')
		await page.screenshot({path: 'screenshots/screenshot.png'})
	} finally {
		await page.close()
	}
}