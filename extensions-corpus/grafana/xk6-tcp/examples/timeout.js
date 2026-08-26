import { Socket } from "k6/x/tcp";

/**
 * Timeout handling example.
 * Demonstrates setting and handling read timeouts for idle connections.
 */
export default async function () {
    const socket = new Socket();
    const closed = new Promise((resolve) => {
        socket.on("close", () => {
            console.log("Connection closed");
            resolve();
        });
    });

    socket.on("data", (data) => {
        const response = String.fromCharCode.apply(null, new Uint8Array(data));
        console.log("Received data:", response);

        // Reset timeout after receiving data
        socket.setTimeout(5000);
    });

    socket.on("timeout", () => {
        console.log("Connection timeout - no data received for 5 seconds");
        // You must manually close after timeout
        socket.destroy();
    });

    socket.on("error", (err) => {
        console.error("Socket error:", err);
    });

    const host = __ENV.TCP_ECHO_HOST || "localhost";
    const port = __ENV.TCP_ECHO_PORT || "8080";

    await socket.connect(port, host);
    console.log("Connected to server");

    // Set a 5-second timeout for inactivity
    socket.setTimeout(5000);
    console.log("Timeout set to 5 seconds");

    // Optionally send a message with await socket.write("Hello");
    await closed;
}
