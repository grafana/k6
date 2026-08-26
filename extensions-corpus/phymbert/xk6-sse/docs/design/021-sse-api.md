# Introduce an SSE API module for k6

|                      |                                                         |
|:---------------------|:--------------------------------------------------------|
| **author**           | @phymbert                                               |
| **status**           | 🔧 proposal                                             |
| **revisions**        | [initial](https://github.com/grafana/k6/pull/3639)      |
| **Proof of concept** | [branch](https://github.com/phymbert/xk6-sse/tree/main) |
| **references**       | [#746](https://github.com/grafana/k6/issues/746)        |

## Problem definition

The current version of k6 reads the full http response body before returning to the client,
which make impossible testing [Server-Sent Event](https://fr.wikipedia.org/wiki/Server-sent_events). 

We propose to introduce a new `sse` module.
This module is intended to offer an intuitive and user-friendly API for SSE interactions within k6 scripts.

## Proposed solution

We suggest to implement a minimalist, experimental (`sse`) module based on the go http client.
The new module will allow users to interact with SSE.
The module will provide an `open` which will allow the user to pass a setup function to configure an `event` callback as it is done in the `ws` module with `message`.

### Limitation
The module will not support async io and the javascript main loop will be blocked during the http request duration.

### Example usage

```javascript
import sse from 'k6/experimental/sse';

var url = "https://echo.websocket.org/.sse";
var params = {"tags": {"my_tag": "hello"}};

var response = sse.open(url, params, function (client) {
    client.on('open', function open() {
        console.log('connected');
    });

    client.on('event', function (event) {
        console.log(`event id=${event.id}, name=${event.name}, data=${event.data}`);
    });

    client.on('error', function (e) {
        console.log('An unexpected error occurred: ', e.error());
    });
});

check(response, {"status is 200": (r) => r && r.status === 200});
```

### Conclusion

We believe the [proof of concept](https://github.com/grafana/k6/blob/d5cd1010ecb2381376188c8a47ab861cf8b5dc3d/js/modules/k6/experimental/sse/sse.go) developed with this proposal illustrates the feasibility and benefits of developing such an API.
