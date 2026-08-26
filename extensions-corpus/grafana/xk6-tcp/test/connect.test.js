const fail = require("k6/execution").test.fail
const assert = (condition, message) => { if (!condition) { fail(message) } }

const tcp = require("k6/x/tcp")

exports.default = async () => {
  const socket = new tcp.Socket({})

  let connectHandlerCalled = false
  socket.on("connect", () => {
    assert(socket.connected, "socket should be connected after connect event")

    connectHandlerCalled = true

    socket.destroy()
  })

  let closeHandlerCalled = false
  const prom = new Promise((resolve) => {
    socket.on("close", () => {
      closeHandlerCalled = true
      resolve()
    })
  })

  assert(!socket.connected, "socket should not be connected initially")

  await socket.connect(__ENV.TCP_ECHO_PORT, __ENV.TCP_ECHO_HOST)
  await prom

  assert(connectHandlerCalled, "connect handler was not called")
  assert(closeHandlerCalled, "close handler was not called")
}
