import { Socket } from "k6/x/tcp";

/**
 * Binary data example demonstrating ArrayBuffer usage.
 * Shows how to send and receive binary protocol data.
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
        const bytes = new Uint8Array(data);
        console.log("Received binary data:", Array.from(bytes));

        // Parse binary response
        if (bytes.length >= 2) {
            const version = bytes[0];
            const length = bytes[1];
            const payload = bytes.slice(2);
            const text = String.fromCharCode.apply(null, payload);

            console.log(`Version: 0x${version.toString(16)}, Length: ${length}`);
            console.log(`Payload: ${text}`);
        }

        socket.destroy();
    });

    socket.on("error", (err) => {
        console.error("Socket error:", err);
    });

    // Example: Send binary protocol header
    // Format: [0x01, length, ...data]
    const message = "Binary data";
    const messageBytes = Array.from(message).map(c => c.charCodeAt(0));

    const packet = new Uint8Array(2 + messageBytes.length);
    packet[0] = 0x01; // Protocol version
    packet[1] = messageBytes.length; // Length field
    packet.set(messageBytes, 2); // Data

    const host = __ENV.TCP_ECHO_HOST || "localhost";
    const port = __ENV.TCP_ECHO_PORT || "8080";

    await socket.connect(port, host);
    console.log("Connected to server");
    console.log("Sending binary packet:", Array.from(packet));
    await socket.write(packet.buffer);
    await closed;
}
