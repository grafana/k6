const fail = require("k6/execution").test.fail
const assert = (condition, message) => { if (!condition) { fail(message) } }

const tcp = require("k6/x/tcp")

exports.default = async () => {
  {
    const socket = new tcp.Socket()
    let rejected = false

    try {
      await socket.connect(true)
    } catch (_) {
      rejected = true
    }

    socket.destroy()
    assert(rejected, "connect should reject on invalid argument type")
  }

  {
    const socket = new tcp.Socket()
    let rejected = false

    try {
      await socket.connect(0, "127.0.0.1")
    } catch (_) {
      rejected = true
    }

    socket.destroy()
    assert(rejected, "connect should reject on connection failure")
  }

  {
    const socket = new tcp.Socket()
    let rejected = false

    socket.on("error", () => {})

    try {
      await socket.connect(0, "127.0.0.1")
    } catch (_) {
      rejected = true
    }

    socket.destroy()
    assert(rejected, "connect should reject on connection failure even with an error handler")
  }

  {
    const socket = new tcp.Socket()
    let rejected = false

    socket.on("error", () => {})

    try {
      await socket.connect(true)
    } catch (_) {
      rejected = true
    }

    socket.destroy()
    assert(rejected, "connect should reject on invalid argument type even with an error handler")
  }
}
