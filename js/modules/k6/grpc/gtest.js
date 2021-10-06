import grpc from "k6/net/grpc";

var client = new grpc.Client();
// client.load([], "../../../../../grpc-go/examples/helloworld/helloworld/helloworld.proto");

export default function () {
        if (!client) {
                throw new Error("no client created");
        }
        client.connect('localhost:50051', { plaintext: true, timeout: '3s', reflect: true });
        var resp = client.invoke('/helloworld.Greeter/SayHello', {})
        if (!resp.message || resp.error ) {
                throw new Error('unexpected response message: ' + JSON.stringify(resp.message))
        }
        console.log(JSON.stringify(resp), resp.error)
}