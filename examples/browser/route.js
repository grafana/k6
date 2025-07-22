import { check, sleep } from "k6";
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

	await page.route(/(\.png$)|(\.jpg$)/, function (route) {
		console.log("Blocking image request");
		route.abort();
	});

	// await page.route(
	// 	"https://jsonplaceholder.typicode.com/todos/1",
	// 	function (route) {
	// 		console.log("Intercepting request to external API");
	// 		route.fulfill({
	// 			status: 200,
	// 			body: JSON.stringify({
	// 				id: 1,
	// 				title: "Test Todo",
	// 				completed: false,
	// 			}),
	// 			contentType: "application/json",
	// 			headers: {
	// 				"Access-Control-Allow-Origin": "*",
	// 				"Access-Control-Allow-Credentials": "true",
	// 			},
	// 		});
	// 	}
	// );

	await page.route(
		"https://jsonplaceholder.typicode.com/todos/1",
		function (route) {
			console.log("Intercepting request to external API");
			route.continue({
				url: "https://jsonplaceholder.typicode.com/todos/1",
			});
		}
	);

	await page.goto("http://localhost:3333/browser.php");

	const button = await page.getByRole("button").nth(4);
	console.log(await button.textContent());
	await button.click();

	sleep(1);

	const checkData = await page
		.locator("#external-data-display")
		.textContent();
	check(page, {
		externalData: checkData === "External data: Test Todo",
	});

	sleep(3);

	await page.close();
}
