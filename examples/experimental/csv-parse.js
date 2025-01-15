import { open } from 'k6/experimental/fs'
import csv from 'k6/experimental/csv'
import { scenario } from 'k6/execution'

export const options = {
	iterations: 10,
}

// Open the csv file, and parse it ahead of time.
const file = await open('data.csv');
// The `csv.parse` function consumes the entire file at once, and returns
// the parsed records as a SharedArray object.
const csvRecords = await csv.parse(file, { delimiter: ',' })

export default async function() {
	// The csvRecords a SharedArray. Each element is a record from the CSV file, represented as an array
	// where each element is a field from the CSV record.
	//
	// Thus, `csvRecords[scenario.iterationInTest]` will give us the record for the current iteration.
	console.log(csvRecords[scenario.iterationInTest])
}

