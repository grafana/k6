const fail = require("k6/execution").test.fail
const assert = (condition, message) => { if (!condition) { fail(message) } }

const tcp = require("k6/x/tcp")

const bytesToString = (data) => String.fromCharCode.apply(null, new Uint8Array(data))

async function expectEchoed(encoded, options, expected) {
  const socket = new tcp.Socket()
  let received = false

  socket.on("data", (data) => {
    received = true

    const actual = bytesToString(data)
    assert(actual === expected, `expected '${expected}', got '${actual}'`)
    socket.destroy()
  })

  socket.on("error", (err) => {
    fail(`unexpected socket error: ${err}`)
  })

  const closed = new Promise((resolve) => {
    socket.on("close", () => {
      resolve()
    })
  })

  await socket.connect(__ENV.TCP_ECHO_PORT, __ENV.TCP_ECHO_HOST)
  await socket.write(encoded, options)
  await closed

  assert(received, "data handler was not called")
}

async function expectWriteRejected(data, options, registerErrorHandler) {
  const socket = new tcp.Socket()
  let rejected = false
  let errorFired = false
  let errorPromise = Promise.resolve()

  if (registerErrorHandler) {
    errorPromise = new Promise((resolve) => {
      socket.on("error", () => {
        errorFired = true
        resolve()
      })
    })
  }

  try {
    await socket.write(data, options)
  } catch (_) {
    rejected = true
  }

  await errorPromise
  socket.destroy()

  assert(rejected, "write should reject")
  if (registerErrorHandler) {
    assert(errorFired, "error handler was not called")
  }
}

exports.default = async () => {
  await expectEchoed("Hello", {}, "Hello")
  await expectEchoed("Hello", { encoding: "utf8" }, "Hello")
  await expectEchoed("Hello", { encoding: "utf-8" }, "Hello")
  await expectEchoed("Hello", { encoding: "ascii" }, "Hello")
  await expectEchoed("SGVsbG8=", { encoding: "base64" }, "Hello")
  await expectEchoed("SGVsbG8", { encoding: "base64url" }, "Hello")
  await expectEchoed("SGVsbG8=", { encoding: "base64url" }, "Hello")
  await expectEchoed("48656c6c6f", { encoding: "hex" }, "Hello")

  {
    const socket = new tcp.Socket()
    let received = false

    socket.on("data", (data) => {
      received = true
      assert(bytesToString(data) === "Hello", `expected 'Hello', got '${bytesToString(data)}'`)
      socket.destroy()
    })

    socket.on("error", (err) => {
      fail(`unexpected socket error: ${err}`)
    })

    const closed = new Promise((resolve) => {
      socket.on("close", () => {
        resolve()
      })
    })

    await socket.connect(__ENV.TCP_ECHO_PORT, __ENV.TCP_ECHO_HOST)
    await socket.write(new Uint8Array([72, 101, 108, 108, 111]).buffer, { encoding: "hex" })
    await closed

    assert(received, "ArrayBuffer data handler was not called")
  }

  await expectWriteRejected("Hello", { encoding: "latin1" }, true)
  await expectWriteRejected("not!!base64", { encoding: "base64" }, false)
  await expectWriteRejected("zzzz", { encoding: "hex" }, false)
}
