
import http from "k6/http";
import { records } from "./somefile.js"

export const options = {
	vus: 100,
	iterations: 100,
};

export default function () {
	if (!Array.isArray(records) || records.length === 0) {
		throw new Error("failed to load JSON without SharedArray");
	}
	const row = records[(__VU + __ITER) % records.length];
	if (!row || !row.email) {
		throw new Error("invalid JSON row");
	}
}
