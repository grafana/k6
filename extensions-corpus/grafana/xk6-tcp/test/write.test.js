const fail = require("k6/execution").test.fail
const assert = (condition, message) => { if (!condition) { fail(message) } }

const tcp = require("k6/x/tcp")

exports.default = async () => {
  const socket = new tcp.Socket({})

  let dataHandlerCalled = false
  socket.on("data", (data) => {
    dataHandlerCalled = true

    const str = String.fromCharCode.apply(null, new Uint8Array(data))

    assert(str === "Hey there", `data mismatch, expected 'Hey there', got '${str}'`)
    socket.destroy()
  })

  const prom = new Promise((resolve) => {
    socket.on("close", () => {
      resolve()
    })
  })

  socket.on("error", (err) => {
    console.log(`Socket error: ${err}`)
  })

  await socket.connect(__ENV.TCP_ECHO_PORT, __ENV.TCP_ECHO_HOST)
  await socket.write("Hey there")
  await prom

  assert(dataHandlerCalled, "data handler was not called")
}
