import http from "k6/http";
import { open } from "k6/experimental/fs";

export const options = {
	vus: 1,
	iterations: 1,
};

// As k6 does not support asynchronous code in the init context, yet, we need to
// use a top-level async function to be able to use the `await` keyword.
let file;
(async function () {
	file = await open("bonjour.txt");
})();

export default async function () {
	// Obtain information about the file.
	const fileinfo = await file.stat();
	if (fileinfo.name != "bonjour.txt") {
		throw new Error("Unexpected file name");
	}

	let res = http.post(
		"https://httpbin.test.k6.io/post",
		await file.readAll(),
		{
			headers: { "Content-Type": "text/plain" },
		}
	);

	console.log(res.body);
}
