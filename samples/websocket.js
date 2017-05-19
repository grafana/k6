import websocket from "k6/websocket";

export default function () {
    var result = websocket.connect("wss://echo.websocket.org", function(socket) {
        socket.on('open', function open() {
            console.log('connected');
            socket.send(Date.now());

            socket.setInterval(function timeout() {
                socket.ping();
                console.log("Pinging every 1sec (setInterval test)");
            }, 1000);
        });

        socket.on('pong', function () {
            console.log("PONG!");
        });

        socket.on('message', function incoming(data) {
            console.log(`Roundtrip time: ${Date.now() - data} ms`);
            socket.setTimeout(function timeout() {
                socket.send(Date.now());
            }, 500);
        });

        socket.on('close', function close() {
            console.log('disconnected');
        });

        socket.on('error', function (e) {
            console.log('An error occured: ', e.error());
        });

        socket.setTimeout(function() {
            console.log('5 seconds passed, closing the socket');
            socket.close();
        }, 5000);
    });
};
