import { sleep } from "k6";
import Papa from "https://cdn.jsdelivr.net/npm/papaparse/papaparse.js";

export const options = {
	vus: 100,
	iterations:100,
};

const csvText = open("./data.csv");
const parsed = Papa.parse(csvText, { header: true, skipEmptyLines: true });
const rows=  parsed.data;


if (!Array.isArray(rows) || rows.length === 0) {
	throw new Error("failed to parse CSV with PapaParse");
}

export default function () {
	const row = rows[(__VU + __ITER) % rows.length];
	if (!row || !row.email) {
		throw new Error("invalid CSV row");
	}
}
