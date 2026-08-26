import tcp from "k6/x/tcp"

export default async function () {
  const socket = new tcp.Socket()
  const closePromise = new Promise((resolve) => {
    socket.on("close", () => {
      console.log("Connection closed")
      resolve()
    })
  })

  socket.on("data", (data) => {
    console.log("Received data")
    const str = String.fromCharCode.apply(null, new Uint8Array(data))
    console.log(str)
    socket.destroy()
  })

  socket.on("error", (err) => {
    console.log(`Socket error: ${err}`)
  })

  await socket.connect(__ENV.TCP_ECHO_PORT, __ENV.TCP_ECHO_HOST)
  console.log("Connected")
  await socket.write("Hey there\n")

  await closePromise
}
