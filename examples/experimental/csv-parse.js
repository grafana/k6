import { open } from 'k6/experimental/fs'
import csv from 'k6/experimental/csv'
import { scenario } from 'k6/execution'

export const options = {
	iterations: 10,
}

// Open the csv file, and parse it ahead of time.
let file;
let csvRecords;
(async function () {
	file = await open('data.csv');

	// The `csv.parse` function consumes the entire file at once, and returns
	// the parsed records as a SharedArray object.
	csvRecords = await csv.parse(file, {delimiter: ','})
})();


export default async function() {
	console.log(csvRecords[scenario.iterationInTest])
}

