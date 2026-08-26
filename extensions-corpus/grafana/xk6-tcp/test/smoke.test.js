const fail = require("k6/execution").test.fail
const assert = (condition, message) => { if (!condition) { fail(message) } }

const tcp = require("k6/x/tcp")

exports.default = () => {
  const socket = new tcp.Socket()

  assert(typeof socket === "object", "socket should be an object")

  socket.destroy()
}
