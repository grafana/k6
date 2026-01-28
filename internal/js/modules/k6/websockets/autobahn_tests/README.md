# Autobahn test suite

Docs: https://github.com/crossbario/autobahn-testsuite

## Usage

Run the WebSocket server.

```sh
$ docker run -it --rm \
    -v ${PWD}/config:/config \
    -v ${PWD}/reports:/reports \
    -p 9001:9001 \
    -p 8080:8080 \
    --name fuzzingserver \
    crossbario/autobahn-testsuite
```

Run the autobahn client test with k6.

```sh
./k6 run ./script.js
```

Open the browser to `http://localhost:8080` for checking the report.
