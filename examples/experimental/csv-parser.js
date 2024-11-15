import { open } from 'k6/experimental/fs'
import csv from 'k6/experimental/csv'

export const options = {
	iterations: 10,
}

const file = await open('data.csv');;
const parser = new csv.Parser(file);;

export default async function() {
	// The parser `next` method attempts to read the next row from the CSV file.
	//
	// It returns an iterator-like object with a `done` property that indicates whether
	// there are more rows to read, and a `value` property that contains the row fields
	// as an array.
	const { done, value } = await parser.next();
	if (done) {
		throw new Error("No more rows to read");
	}

	// We expect the `value` property to be an array of strings, where each string is a field
	// from the CSV record.
	console.log(done, value);
}
