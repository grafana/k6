import streams from "k6/streams"

export function setup() {
  // Args: filename, loop, hasHeader, startPos (byte)
  var f1 = streams.openFile("citypopsmall.csv", true, true, 0)
  var f2 = streams.openFile("streams.k6.js", true, false, 5)
  let headers = streams.file(f1).getHeaders().join("\t")
  console.log(f1)
  console.log(headers)
  return [headers, f1, f2]
}

export default function([headers, f1, f2]) {
  let line = streams.file(f1).readCSVLine()
  console.log(line.join("\t"))
  // Read line from f2
  line = streams.file(f2).readLine()
  console.log(line)
}

export function teardown([_, f1, f2]) {
  streams.file(f2).close()
  streams.file(f1).close()
}
