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
	// About information about the file
	const fileinfo = await file.stat();
	if (fileinfo.name != "bonjour.txt") {
		throw new Error("Unexpected file name");
	}

	// Define a buffer of the same size as the file
	// to read the file content into.
	const buffer = new Uint8Array(fileinfo.size);

	// Read the file's content into the buffer
	const bytesRead = await file.read(buffer);

	// Check that we read the expected number of bytes
	if (bytesRead != fileinfo.size) {
		throw new Error("Unexpected number of bytes read");
	}

	// Seek back to the beginning of the file
	await file.seek(0);
}
