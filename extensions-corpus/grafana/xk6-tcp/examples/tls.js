import { Socket } from "k6/x/tcp";

/**
 * TLS/SSL connection example.
 * Demonstrates secure TCP connections with TLS encryption and a full HTTP response.
 */

export const options = {
    // Optional: Configure TLS settings
    // insecureSkipTLSVerify: true, // Skip certificate verification (dev/test only)
};

export default async function () {
    const socket = new Socket({
        tags: {
            protocol: "tls",
        },
    });

    let fullResponse = "";

    // Set up event handlers before connecting
    const dataPromise = new Promise((resolve) => {
        socket.on("data", (data) => {
            const chunk = String.fromCharCode.apply(null, new Uint8Array(data));
            fullResponse += chunk;
            console.log("Received data chunk, length:", data.byteLength);

            // Stop waiting once we likely have the full HTML response
            if (chunk.includes("</html>")) {
                resolve();
            }
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

    // Connect to the TLS endpoint
    const host = __ENV.TLS_HOST || "example.com";
    const port = parseInt(__ENV.TLS_PORT || "443");

    await socket.connect({
        port: port,
        host: host,
        tls: true,
    });
    console.log("TLS connection established");

    // Send an HTTP request over the encrypted connection
    await socket.write(
        `GET / HTTP/1.1\r\nHost: ${host}\r\nConnection: close\r\n\r\n`
    );
    console.log("Request sent, waiting for response...");

    // Wait for the response, but do not hang forever on slow endpoints
    await Promise.race([
        dataPromise,
        new Promise((resolve) => setTimeout(resolve, 2000))
    ]);

    console.log("Response received, length:", fullResponse.length);
    if (fullResponse.length > 200) {
        console.log("First 200 chars:", fullResponse.substring(0, 200));
    } else {
        console.log("Full response:", fullResponse);
    }

    if (fullResponse.includes("HTTP/") && fullResponse.includes("html")) {
        console.log("Successfully received an HTTP/1.1 HTML response");
    } else if (fullResponse.length === 0) {
        console.log("No response received before cleanup");
    }

    socket.destroy();
    await closePromise;
}
