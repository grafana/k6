// Import the necessary k6 gRPC module and the k6 check module.

import grpc from 'k6/net/grpc';
import {check} from "k6";

// Create a new gRPC client.

let client = new grpc.Client();

// Define the default k6 script.

export default () => {
    // Connect to the gRPC server at "127.0.0.1:10000" using plaintext communication.
    // The "reflect" option allows the server to send the request back to the client, which can be useful for debugging.

    client.connect("127.0.0.1:10000", { plaintext: true, reflect: true });

    // Make a request to the "main.FeatureExplorer/GetFeature" gRPC method with specific latitude and longitude values.

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
