import { Socket } from "k6/x/tcp";
import { sleep } from "k6";

export default async function () {
    const socket = new Socket();

    const closePromise = new Promise((resolve) => {
        socket.on("close", () => {
            console.log("Connection closed");
            resolve();
        });
    });

    socket.on("connect", () => {
        console.log("TLS connection established!");
    });

    socket.on("error", (err) => {
        console.error("Error:", err);
    });

    // Connect to example.com on port 443 with TLS
    await socket.connect({
        port: 443,
        host: "example.com",
        tls: true,
    });

    // Give the connection a moment, then close
    sleep(0.1);
    socket.destroy();

    await closePromise;
    console.log("Test completed");
}
