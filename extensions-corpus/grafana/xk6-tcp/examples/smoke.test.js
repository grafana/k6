import { Socket } from "k6/x/tcp";
import { check } from "k6";

/**
 * Smoke test example demonstrating k6 load testing with TCP sockets.
 * This shows how to use xk6-tcp in a real load test scenario with async/await.
 */

export const options = {
    vus: 5,
    duration: "10s",
    thresholds: {
        checks: ["rate>0.9"],
        tcp_socket_duration: ["p(95)<1000"], // 1 second
        tcp_errors: ["count==0"],
    },
};

export default async function () {
    const socket = new Socket({
        tags: {
            test: "smoke",
        },
    });

    let receivedData = "";

    // Set up promise for data reception
    const dataPromise = new Promise((resolve) => {
        socket.on("data", (data) => {
            receivedData = String.fromCharCode.apply(null, new Uint8Array(data));
            resolve();
        });
    });

    // Set up close promise
    const closePromise = new Promise((resolve) => {
        socket.on("close", () => {
            resolve();
        });
    });

    socket.on("error", (err) => {
        console.error("Socket error:", err);
    });

    // Connect and send message
    const host = __ENV.TCP_ECHO_HOST || "localhost";
    const port = __ENV.TCP_ECHO_PORT || "8080";

    await socket.connect(port, host);
    await socket.write(`Message from VU ${__VU} iteration ${__ITER}`);

    // Wait for response
    await dataPromise;
    socket.destroy();
    await closePromise;

    // Perform checks after all operations complete
    check(null, {
        "received echo response": () => receivedData.length > 0,
        "echo matches sent data": () =>
            receivedData.includes(`Message from VU ${__VU} iteration ${__ITER}`),
    });
}
