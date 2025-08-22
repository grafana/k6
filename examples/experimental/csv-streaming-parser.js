import csv from 'k6/experimental/csv'

export const options = {
	scenarios: {
		default: {
			executor: "shared-iterations",
			vus: 1,
			iterations: 100,
		}
	}
};

// Use the new StreamingParser for large CSV files to avoid memory issues
// This takes a file path string instead of a fs.File object and streams the data
const parser = new csv.StreamingParser('data.csv', {
	skipFirstLine: true,
});

export default async function() {
	// The streaming parser `next` method works the same as the regular parser
	// but doesn't load the entire file into memory during initialization
	const { done, value } = await parser.next();
	if (done) {
		throw new Error("No more rows to read");
	}

	// We expect the `value` property to be an array of strings, where each string is a field
	// from the CSV record.
	console.log(done, value);
}

// Optional: Clean up resources when the test finishes
export function teardown() {
	// The streaming parser can be explicitly closed to free resources
	// though it will be closed automatically when the test ends
	parser.close();
} 