const f1 = openStream("citypopsmall.csv", true, false, 0, "csv")
const f2 = openStream("citypopsmall.csv", true, false, 0)

export default function() {
  console.log(__VU, f1.readCSVLine().join('\t'))
  console.log(__VU, f2.readCSVLine())
}
