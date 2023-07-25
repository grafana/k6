import { open, SeekMode } from "k6/experimental/fs";

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

	// Define a buffer of the same size as the file to
	// read the file content into.
	const buffer = new Uint8Array(fileinfo.size);

	// Read the file content into the buffer.
	const bytesRead = await file.read(buffer);

	// Check that we read the expected number of bytes.
	if (bytesRead != fileinfo.size) {
		throw new Error("Unexpected number of bytes read");
	}

	const offset = await file.seek(0, SeekMode.Start);
	console.log(`Seeked to offset ${offset}`);
}
