import { Socket } from "k6/x/tcp";

/**
 * Simple TCP socket example.
 */
export default async function () {
    const socket = new Socket();

    // Set up event handlers before connecting
    const dataPromise = new Promise((resolve) => {
        socket.on("data", (data) => {
            const response = String.fromCharCode.apply(null, new Uint8Array(data));
            console.log("Received:", response);
            resolve(response);
        });
    });

    const closePromise = new Promise((resolve) => {
        socket.on("close", () => {
            console.log("Connection closed");
            resolve();
        });
    });

    socket.on("error", (err) => {
        console.error("Socket error:", err);
    });

    // Connect to the echo server
    const host = __ENV.TCP_ECHO_HOST || "localhost";
    const port = __ENV.TCP_ECHO_PORT || "8080";

    await socket.connect(port, host);
    console.log("Connected to TCP server");

    // Send a message
    await socket.write("Hello, TCP!");
    console.log("Message sent");

    // Wait for the response before cleaning up
    await dataPromise;

    // Clean up
    socket.destroy();
    await closePromise;
}
