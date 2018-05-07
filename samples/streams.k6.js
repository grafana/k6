import streams from "k6/streams"

export function setup() {
  // Args: filename, loop, hasHeader, startPos (byte)
  var f1 = streams.openFile("citypopsmall.csv", true, true, 0)
  console.log(f1)
  return f1
}

export default function(f1) {
  console.log(f1)
  let headers = streams.file(f1).getHeaders()
  let line = streams.file(f1).readCSVLine()
  for( var l in line ) {
    console.log(headers[l], line[l])
  }
}

export function teardown(f1) {
  streams.file(f1).close()
}
