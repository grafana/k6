import { open } from 'k6/experimental/fs'
import csv from 'k6/experimental/csv'

export const options = {
	iterations: 10,
}

let file;
let parser;
(async function () {
	file = await open('data.csv');
	parser = new csv.Parser(file);
})();

export default async function() {
	const {done, value} = await parser.next();
	if (done) {
		throw new Error("No more rows to read");
	}

	console.log(value);
}
