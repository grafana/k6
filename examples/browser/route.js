import { sleep } from "k6";
import { browser } from "k6/browser";

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
	},
};

export default async function () {
	const page = await browser.newPage();

	// registers a handler that logs all requests made by the page
	await page.route("https://quickpizza.grafana.com/api/tools", function (route) {
		console.log(route.request().url());
		route.abort();
	});
	
	page.on("request", function (request) {
		const url = request.url();
		if (url.includes("/api/tools")) {
			console.log("on request", "request url", request.url());
		}
	});

	page.on("response", function (response) {
		const url = response.url();
		if (url.includes("/api/tools")) {
			console.log("on response", "response url", response.url());
		}
	});

	await page.goto("https://quickpizza.grafana.com/", {
		waitUntil: "networkidle",
	});

	await page.close();
}
