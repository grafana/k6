const fail = require("k6/execution").test.fail
const assert = (condition, message) => { if (!condition) { fail(message) } }

const tcp = require("k6/x/tcp")

exports.default = async () => {
    const socket = new tcp.Socket({})

    let timeoutHandlerCalled = false
    socket.on("timeout", () => {
        timeoutHandlerCalled = true
        // User must manually destroy after timeout
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
    socket.setTimeout(2000)
    await prom

    assert(timeoutHandlerCalled, "timeout handler was not called")
}
