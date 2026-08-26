const fail = require("k6/execution").test.fail
const assert = (condition, message) => { if (!condition) { fail(message) } }

const tcp = require("k6/x/tcp")

async function expectStringPortAccepted(connectCall, label) {
  const socket = new tcp.Socket()
  let rejected = false
  let errorText = ""

  try {
    await connectCall(socket)
  } catch (err) {
    rejected = true
    errorText = String(err)
  }

  socket.destroy()

  assert(rejected, `${label} should reject on connection failure`)
  assert(!errorText.includes("invalid type"), `${label} should not fail type validation: ${errorText}`)
}

exports.default = async () => {
  {
    const socket = new tcp.Socket()
    const result = socket.on("connect", () => {})

    assert(result === undefined, `on() should return undefined, got ${result}`)
    socket.destroy()
  }

  {
    const socket = new tcp.Socket()
    const result = socket.destroy()

    assert(result === undefined, `destroy() should return undefined, got ${result}`)
  }

  await expectStringPortAccepted(
    (socket) => socket.connect("0", "127.0.0.1"),
    "connect(port: string, host)",
  )

  await expectStringPortAccepted(
    (socket) => socket.connect({ port: "0", host: "127.0.0.1" }),
    "connect({ port: string, host })",
  )
}
