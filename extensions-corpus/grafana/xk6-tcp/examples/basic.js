import { Socket } from "k6/x/tcp";

/**
 * Minimal TCP socket example.
 * This is the simplest possible usage - create a socket, connect, and close.
 */
export default async function () {
  const socket = new Socket();
  const closed = new Promise((resolve) => {
    socket.on("close", () => {
      console.log("Closed!");
      resolve();
    });
  });

  socket.on("error", (err) => {
    console.error("Error:", err);
  });

  const host = __ENV.TCP_ECHO_HOST || "localhost";
  const port = __ENV.TCP_ECHO_PORT || "8080";

  await socket.connect(port, host);
  console.log("Connected!");
  socket.destroy();

  await closed;
}
