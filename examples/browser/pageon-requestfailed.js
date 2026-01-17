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

    // Register a handler for failed requests (e.g., DNS errors, connection refused)
    page.on('requestfailed', request => {
        const failure = request.failure()
        console.log(`Request failed: ${request.url()}`)
        console.log(`  Method: ${request.method()}`)
        console.log(`  Error: ${failure ? failure.errorText : 'unknown'}`)
    })

    try {
        // This will trigger a requestfailed event due to DNS failure
        await page.goto('https://does-not-exist.invalid/')
    } catch (e) {
        // Navigation error expected
    }

    await page.close()
}
