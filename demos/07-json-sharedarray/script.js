import { sleep } from "k6";
import { SharedArray } from "k6/data";

export const options = {
  vus: 100,
  iterations: 100,
};

const records = new SharedArray("demo-json-shared", function () {
  return JSON.parse(open("./data.json"));
});

if (!Array.isArray(records) || records.length === 0) {
  throw new Error("failed to load JSON in SharedArray");
}

export default function () {
  const row = records[(__VU + __ITER) % records.length];
  if (!row || !row.email) {
    throw new Error("invalid JSON row");
  }
}
