const fail = require("k6/execution").test.fail
const assert = (condition, message) => { if (!condition) { fail(message) } }

const tcp = require("k6/x/tcp")

exports.default = async () => {
  const socket = new tcp.Socket({})
  const events = []
  let sawData = false

  const closed = new Promise((resolve) => {
    socket.on("close", () => {
      events.push("close")
      resolve()
    })
  })

  socket.on("connect", () => {
    events.push("connect")
  })

  socket.on("data", () => {
    if (sawData) {
      return
    }

    sawData = true
    events.push("data")
    socket.destroy()
  })

  socket.on("error", (err) => {
    fail(`unexpected socket error: ${err}`)
  })

  await socket.connect(__ENV.TCP_ECHO_PORT, __ENV.TCP_ECHO_HOST)
  await socket.write("event order")
  await closed

  const actual = events.join(" -> ")
  const dataIndex = events.indexOf("data")
  const closeIndex = events.indexOf("close")

  assert(events[0] === "connect", `expected connect first, got ${actual}`)
  assert(dataIndex > 0, `expected data after connect, got ${actual}`)
  assert(closeIndex > dataIndex, `expected close after data, got ${actual}`)
}
