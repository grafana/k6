import csv from "k6/encoding/csv"

const f1 = openStream("citypopsmall.csv", 0, "csv")
const f2 = openStream("citypopsmall.csv", 0)
const c1 = csv.newStream(f1, true, true)
const headers = c1.getHeaders()

export default function() {
  if (__ITER == 0) {
    console.log(headers)
  }
  console.log(__VU, c1.readCSVLine().join('\t'))
  console.log(__VU, f2.readLine())
}
