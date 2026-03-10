import { sleep } from "k6";
import { open } from "k6/experimental/fs";
import csv from "k6/experimental/csv";

export const options = {
	vus: 100,
	iterations: 100,
};

const file = await open("./data.csv");
const records = await csv.parse(file, { delimiter: "," });

if (!records || records.length === 0) {
	throw new Error("failed to parse CSV with experimental/csv");
}

export default function () {
}
