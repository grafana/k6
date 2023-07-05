import { open } from "k6/experimental/fs";

export const options = {
	vus: 100,
	iterations: 1000,
};

// As k6 does not support asynchronous code in the init context, yet, we need to
// use a top-level async function to be able to use the `await` keyword.
let file;
(async function () {
	file = await open("bonjour.txt");
})();

export default async function () {
	const fileinfo = await file.stat();
	if (fileinfo.name != "bonjour.txt") {
		throw new Error("Unexpected file name");
	}
}
