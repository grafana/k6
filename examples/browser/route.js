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

	await page.route(
		"https://quickpizza.grafana.com/api/tools",
		function (route) {
			route.abort();
		}
	);

	await page.route(/(\.png$)|(\.jpg$)/, function (route) {
		route.abort();
	});

	page.on("request", function (request) {
		console.log("on request", "url", request.url());
	});

	page.on("response", function (response) {
		console.log("on response", "url", response.url());
	});

	await page.goto("https://quickpizza.grafana.com/", {
		waitUntil: "networkidle",
	});

	await page.close();
}
