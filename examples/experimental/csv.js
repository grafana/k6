import { open } from 'k6/experimental/fs'
import csv from 'k6/experimental/csv'

let file;
let parser;
(async function () {
	file = await open('data.csv');
	parser = new csv.Parser(file, {
		delimiter: ',',
		skipFirstLine: true,
		fromLine: 3,
		toLine: 13,
	})
})();

export default async function() {
	while (true) {
		const {done, value} = await parser.next();
		if (done) {
			break;
		}

		console.log(value)
	}
}
