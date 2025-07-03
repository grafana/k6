export const options = {
    vus: 10,
    duration: "30s",
    tags: { env: "test" },
    thresholds: {
        http_req_duration: ["p(95)<500"],
    },
};

export default function () {
    // Simple test function
} 