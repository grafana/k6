// Import gRPC and k6 check module for use in the script.

import grpc from 'k6/net/grpc';
import { check } from "k6";

// Create a new gRPC client and load the protocol buffer definition.

let client = new grpc.Client();
client.load([], "./grpc_server/route_guide.proto");

// Define the default k6 script.

export default () => {
    // Connect to the gRPC server at 127.0.0.1:10000 using plaintext.

    client.connect("127.0.0.1:10000", { plaintext: true });

    // Invoke the "main.FeatureExplorer/GetFeature" method with specific latitude and longitude values.

    const response = client.invoke("main.FeatureExplorer/GetFeature", {
        latitude: 410248224,
        longitude: -747127767
    });

    // Check the response for an "OK" status using the gRPC status code.

    check(response, { "status is OK": (r) => r && r.status === grpc.StatusOK });

    // Log the response message as a JSON string.

    console.log(JSON.stringify(response.message));

    // Close the gRPC client connection.
    client.close();
}

