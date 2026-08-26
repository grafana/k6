import { Socket } from "k6/x/tcp";

/**
 * Socket state inspection example.
 * Demonstrates checking socket connection state and properties.
 */
export default async function () {
    const socket = new Socket();
    const closed = new Promise((resolve) => {
        socket.on("close", () => {
            console.log("Final state:", socket.ready_state);
            console.log("Finally connected:", socket.connected);
            console.log("Total bytes written:", socket.bytes_written);
            console.log("Total bytes read:", socket.bytes_read);
            resolve();
        });
    });

    // Check initial state
    console.log("Initial state:", socket.ready_state);
    console.log("Initially connected:", socket.connected);

    socket.on("data", (data) => {
        const response = String.fromCharCode.apply(null, new Uint8Array(data));
        console.log("Received:", response);

        // Check counters after data transfer
        console.log("Bytes written after write:", socket.bytes_written);
        console.log("Bytes read after receive:", socket.bytes_read);

        socket.destroy();
    });

    socket.on("error", (err) => {
        console.error("Socket error:", err);
        console.log("State on error:", socket.ready_state);
    });

    const host = __ENV.TCP_ECHO_HOST || "localhost";
    const port = __ENV.TCP_ECHO_PORT || "8080";

    await socket.connect(port, host);
    console.log("State after connect:", socket.ready_state);
    console.log("Is connected:", socket.connected);
    console.log("Bytes written:", socket.bytes_written);
    console.log("Bytes read:", socket.bytes_read);
    await socket.write("Test message");
    await closed;
}
