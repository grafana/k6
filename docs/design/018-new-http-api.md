# New HTTP API

## Authors
The k6 core team

## Why is this needed?

The HTTP API (in k6 <=v0.43.0) used by k6 scripts has many limitations, inconsistencies and performance issues, that lead to a poor user experience. Considering that it's the most commonly used JS API, improving it would benefit most k6 users.

The list of issues with the current API is too long to mention in this document, but you can see a detailed list of [GitHub issues labeled `new-http`](https://github.com/grafana/k6/issues?q=is%3Aopen+is%3Aissue+label%3Anew-http) that should be fixed by this proposal, as well as the [epic issue #2461](https://github.com/grafana/k6/issues/2461). Here we'll only mention the relatively more significant ones:

* [#2311](https://github.com/grafana/k6/issues/2311): files being uploaded are copied several times in memory, causing more memory usage than necessary. Related issue: [#1931](https://github.com/grafana/k6/issues/1931)
* [#857](https://github.com/grafana/k6/issues/857), [#1045](https://github.com/grafana/k6/issues/1045): it's not possible to configure transport options, such as proxies or DNS, per VU or group of requests.
* [#761](https://github.com/grafana/k6/issues/761): specifying configuration options globally is not supported out-of-the-box, and workarounds like the [httpx library](https://k6.io/docs/javascript-api/jslib/httpx/) are required.
* [#746](https://github.com/grafana/k6/issues/746): async functionality like Server-sent Events is not supported.
* Related to the previous point, all (except asyncRequest) current methods are synchronous, which is inflexible, and doesn't align with modern APIs from other JS runtimes.
* [#436](https://github.com/grafana/k6/issues/436): the current API is not very friendly or ergonomic. Different methods also have parameters that change places, e.g. `params` is the second argument in `http.get()`, but the third one in `http.post()`.


## Proposed solution(s)

### Design

HTTP is an application-layer protocol that is built upon lower level transport protocols, such as TCP, UDP, or local IPC mechanisms in most operating systems. It's also closely related to the DNS protocol, which all browsers use when resolving a host name before establishing an HTTP connection. As such, we can't implement a flexible and modern HTTP API without exposing APIs for these lower level protocols. In fact, some user requested functionality would be difficult, if not impossible, without access to these APIs (e.g. issues [#1393](https://github.com/grafana/k6/issues/1393), [#1098](https://github.com/grafana/k6/issues/1098), [#2510](https://github.com/grafana/k6/issues/2510), [#857](https://github.com/grafana/k6/issues/857), [#2366](https://github.com/grafana/k6/issues/2366)).

In this sense, we propose designing the HTTP API in such a way that it's built _on top_ of these lower level APIs. By making our networking namespace composable in this way, we open the door for other application-layer protocols to be implemented using the same low level primitives. For example, WebSockets could be implemented on top of the TCP API, gRPC on top of the HTTP/2 API, and so on.

This approach also follows other modern JavaScript runtimes such as Node and Deno, which ensures we're building a familiar and extensible API, instead of a purpose-built library just for HTTP and unique to k6.


With that said, the design of the API should follow these guidelines:

- It should be familiar to users of HTTP APIs from other JS runtimes, and easy for new users to pick up.

  As such, it would serve us well to draw inspiration from existing runtimes and frameworks. Particularly:

  - The [Fetch API](https://developer.mozilla.org/en-US/docs/Web/API/Fetch_API), a [WHATWG standard](https://fetch.spec.whatwg.org/) supported by most modern browsers.
    [Deno's implementation](https://deno.land/manual/examples/fetch_data) and [GitHub's polyfill](https://github.com/github/fetch) are good references to follow.

    This was already suggested in [issue #2424](https://github.com/grafana/k6/issues/2424).

  - The [Streams API](https://developer.mozilla.org/en-US/docs/Web/API/Streams_API), a [WHATWG standard](https://streams.spec.whatwg.org/) supported by most modern browsers.
    [Deno's implementation](https://deno.land/manual@v1.30.3/examples/fetch_data#files-and-streams) is a good reference to follow.

    The work to implement it is tracked in [issue #2978](https://github.com/grafana/k6/issues/2978).

    Streaming files both from disk to RAM to the network, and from network to RAM and possibly disk, would also partly solve our [performance and memory issues with loading large files](https://github.com/grafana/k6/issues/2311).

  - Native support for the [`FormData` API](https://developer.mozilla.org/en-US/docs/Web/API/FormData).

    Currently this is supported with a [JS polyfill](https://k6.io/docs/examples/data-uploads/#advanced-multipart-request), which should be deprecated.

  - Aborting requests or any other async process with the [`AbortSignal`/`AbortController` API](https://developer.mozilla.org/en-US/docs/Web/API/AbortSignal), part of the [WHATWG DOM standard](https://dom.spec.whatwg.org/#aborting-ongoing-activities).

    This is slightly out of scope for the initial phases of implementation, but aborting async processes like `fetch()` is an important feature.

- The Fetch API alone would not address all our requirements (e.g. specifying global and transport options), so we still need more flexible and composable interfaces.

  One source of inspiration is the Go `net/http` package, which the k6 team is already familiar with. Based on this, our JS API could have similar entities:

  - `Dialer`: a low-level interface for configuring TCP/IP options, such as TCP timeout and keep-alive, TLS settings, DNS resolution, IP version preference, etc.

  - `Transport`: interface for configuring HTTP connection options, such as proxies, HTTP version preferences, etc.

    It enables advanced behaviors like intercepting requests before they're sent to the server.

  - `Client`: the main entrypoint for making requests, it encompasses all of the above options. A k6 script should be able to initialize more than one `Client`, each with their separate configuration.

    In order to simplify the API, the creation of a `Client` should use sane defaults for `Dialer` and `Transport`.

  There should be some research into existing JS APIs that offer similar features (e.g. Node/Deno), as we want to offer an API familiar to JS developers, not necessarily Go developers.

  - `Request`/`Response`: represent objects sent by the client, and received from the server. In contrast to the current API, the k6 script should be able to construct `Request` objects declaratively, and then reuse them to make multiple requests with the same (or similar) data.

- All methods that perform I/O calls must be asynchronous. Now that we have `Promise`, event loop and `async`/`await` support natively in k6, there's no reason for these to be synchronous anymore.

- The API should avoid any automagic behavior. That is, it should not attempt to infer desired behavior or options based on some implicit value.

  We've historically had many issues with this ([#878](https://github.com/grafana/k6/issues/878), [#1185](https://github.com/grafana/k6/issues/1185)), resulting in confusion for users, and we want to avoid it in the new API. Even though we want to have sane defaults for most behavior, instead of guessing what the user wants, all behavior should be explicitly configured by the user. In cases where some behavior is ambiguous, the API should raise an error indicating so.


#### Sockets

A Socket represents the file or network socket over which client/server or peer communication happens.

It can be of three types:
- `tcp`: a stream-oriented network socket using the Transmission Control Protocol.
- `udp`: a message-oriented network socket using the User Datagram Protocol.
- `ipc`: a mechanism for communicating between processes on the same machine, typically using files.

The Socket state can either be _active_—meaning connected for a TCP socket, bound for a UDP socket, or open for an IPC socket—, or _inactive_—meaning disconnected, unbound, or closed, respectively.

##### Example

- TCP:
```javascript
import { TCP } from 'k6/x/net';

export default async function () {
  const socket = await TCP.open('192.168.1.1:80', {
                            // default      | possible values
    ipVersion: 0,           // 0            | 4 (IPv4), 6 (IPv6), 0 (both)
    keepAlive: true,        // false        |
    lookup: null,           // dns.lookup() |
    proxy: 'myproxy:3030',  // ''           |
  });
  console.log(socket.active); // true

  // Writing directly to the socket.
  // Requires TextEncoder implementation, otherwise typed arrays can be used as well.
  await socket.write(new TextEncoder().encode('GET / HTTP/1.1\r\n\r\n'));

  // And reading...
  socket.on('data', data => {
    console.log(`received ${data}`);
    socket.close();
  });

  await socket.done();
}
```

- UDP:
```javascript
import { UDP } from 'k6/x/net';

export default async function () {
  const socket = await UDP.open('192.168.1.1:9090');
  await socket.write(new TextEncoder().encode('GET / HTTP/1.1\r\n\r\n'));
  socket.close();
}
```

- IPC:
```javascript
import { IPC } from 'k6/x/net';
import { Client } from 'k6/x/net/http';

export default async function () {
  // The HTTP client supports communicating over a Unix socket.
  const client = new Client({
    dial: async () => {
      return await IPC.open('/tmp/unix.sock');
    },
  });
  await client.get('http://unix/get');

  console.log(client.socket.file.path); // /tmp/unix.sock

  client.socket.close();
}
```

#### Client

An HTTP Client is used to communicate with an HTTP server.

##### Examples

- Using a client with default transport settings, and making a GET request:
```javascript
import { Client } from 'k6/x/net/http';

export default async function () {
  const client = new Client();
  const response = await client.get('https://httpbin.test.k6.io/get');
  const jsonData = await response.json();
  console.log(jsonData);
}
```

- Creating a client with custom transport settings, some HTTP options, and making a POST request:
```javascript
import { TCP } from 'k6/x/net';
import { Client } from 'k6/x/net/http';

export default async function () {
  const client = new Client({
    dial: async address => {
      return await TCP.open(address, { keepAlive: true });
    },
    proxy: 'https://myproxy',
    headers: { 'User-Agent': 'k6' },  // set some global headers
  });
  await client.post('http://10.0.0.10/post', {
    json: { name: 'k6' }, // automatically adds 'Content-Type: application/json' header
  });
}
```

- Configuring TLS with a custom CA certificate and forcing HTTP/2:
```javascript
import { TCP } from 'k6/x/net';
import { Client } from 'k6/x/net/http';
import { open } from 'k6/x/file';

const caCert = await open('./custom_cacert.pem');

export default async function () {
  const client = new Client({
    dial: async address => {
      return await TCP.open(address, {
        tls: {
          alpn: ['h2'],
          caCerts: [caCert],
        }
      });
    },
  });
  await client.get('https://10.0.0.10/');
}
```

- Forcing unencrypted HTTP/2 (h2c):
```javascript
import { TCP } from 'k6/x/net';
import { Client } from 'k6/x/net/http';

export default async function () {
  const client = new Client({
    dial: async address => {
      return await TCP.open(address, { tls: false });
    },
    version: [2],
  });
  await client.get('http://10.0.0.10/');
```


#### Host name resolution

Host names can be resolved to IP addresses in several ways:

- Via a static lookup map defined in the script.
- Via the operating system's facilities (`/etc/hosts`, `/etc/resolv.conf`, etc.).
- By querying specific DNS servers.

When connecting to an address using a host name, the resolution can be controlled via the `lookup` function passed to the socket constructor. By default, the mechanism provided by the operating system is used (`dns.lookup()`).

For example:
```javascript
import { TCP } from 'k6/x/net';
import dns from 'k6/x/net/dns';

const hosts = {
  'hostA': '10.0.0.10',
  'hostB': '10.0.0.11',
};

export default async function () {
  const socket = await TCP.open('myhost', {
    lookup: async hostname => {
      // Return either the IP from the static map, or do an OS lookup,
      // or fallback to making a DNS query to specific servers.
      return hosts[hostname] || await dns.lookup(hostname) ||
        await dns.resolve(hostname, {
          rrtype: 'A',
          servers: ['1.1.1.1:53', '8.8.8.8:53'],
        });
    },
  });
}
```

#### Requests and responses

HTTP requests can be created declaratively, and sent only when needed. This allows reusing request data to send many similar requests.

For example:
```javascript
import { Client, Request } from 'k6/x/net/http';

export default async function () {
  const client = new Client({
    headers: { 'User-Agent': 'k6' },  // set some global headers
  });
  const request = new Request('https://httpbin.test.k6.io/get', {
    // These will be merged with the Client options.
    headers: { 'Case-Sensitive-Header': 'somevalue' },
  });
  const response = await client.get(request, {
    // These will override any options for this specific submission.
    headers: { 'Case-Sensitive-Header': 'anothervalue' },
  });
  const jsonData = await response.json();
  console.log(jsonData);
}
```


#### Data streaming

The [Streams API](https://developer.mozilla.org/en-US/docs/Web/API/Streams_API) allows streaming data that is received or sent over the network, or read from or written to the local filesystem. This enables more efficient usage of memory, as only chunks of it need to be allocated at once.

This is a separate project from the HTTP API, tracked in [issue #2978](https://github.com/grafana/k6/issues/2978), and involves changes in other parts of k6. Certain HTTP API functionality, however, depends on this API being available.

An example inspired by [Deno](https://deno.land/manual/examples/fetch_data#files-and-streams) of how this might work in k6:
```javascript
import { open } from 'k6/x/file';
import { Client } from 'k6/x/net/http';

// Will need supporting await in init context
const file = await open('./logo.svg');  // by default assumes 'read'

export default async function () {
  const client = new Client();
  await client.post('https://httpbin.test.k6.io/post', { body: file.readable });
}
```


#### Fetch API

The [Fetch API](https://developer.mozilla.org/en-US/docs/Web/API/Fetch_API) is a convenience wrapper over existing Client, Socket and other low-level interfaces, with the benefit of being easy to use, and having sane defaults. It's a quick way to fire off some HTTP requests and get some responses, without worrying about advanced configuration.

The implementation in k6 differs slightly from the web API, but we've tried to make it familiar to use wherever possible.

Example:
```javascript
import { fetch } from 'k6/x/net/http';

export default async function () {
  await fetch('https://httpbin.test.k6.io/get');
  await fetch('https://httpbin.test.k6.io/post', {
    // Supports the same options as Client.request()
    method: 'POST',
    headers: { 'User-Agent': 'k6' },
    json: { name: 'k6' },
  });
}
```

#### Events

The new HTTP API will emit events which scripts can subscribe to, in order to implement advanced functionality.

For example, a `requestToBeSent` event is emitted when the request was processed by k6, and just before it is sent to the server. This allows changing the request body or headers, or introducing artificial delays.

```javascript
import { Client } from 'k6/x/net/http';

export default async function () {
  const client = new Client();
  client.on('requestToBeSent', event => {
    console.log(event.type);    // 'requestToBeSent'
    const request = event.data;
    request.headers['Cookie'] = 'somecookie=somevalue;'  // overwrites all previously set cookies
    request.body += ' world!';  // the final body will be 'Hello world!'
  });

  await client.post('https://httpbin.test.k6.io/post', { body: 'Hello' });
}
```

Similarly, a `responseReceived` event is emitted when a response is received from the server, but before it's been fully processed by k6. This can be useful to alter the response in some way, or edit the metrics emitted by k6.

For example:

```javascript
import { Client } from 'k6/x/net/http';

export default async function () {
  const client = new Client();
  let requestID;  // used to correlate a specific response with the request that initiated it

  client.on('requestToBeSent', event => {
    const request = event.data;
    if (!requestID && request.url == 'https://httpbin.test.k6.io/get?name=k6'
        && request.method == 'GET') {
      // The request ID is a UUIDv4 string that uniquely identifies a single request.
      // This is a contrived check and example, but you can imagine that in a complex
      // script there would be many similar requests.
      requestID = request.id;
    }
  });

  client.on('responseReceived', event => {
    const response = event.data;
    if (requestID && response.request.id == requestID) {
      // Change the request duration metric to any value
      response.metrics['http_req_duration'].value = 3.1415;
      // Consider the request successful regardless of its response
      response.metrics['http_req_failed'].value = false;
      // Or drop a single metric
      delete response.metrics['http_req_duration'];
      // Or drop all metrics
      response.metrics = {};
    }
  });

  await client.get('https://httpbin.test.k6.io/get', { query: { name: 'k6' } });
}
```

Event handlers can also be attached directly to a single request/response cycle. This avoids having to correlate responses with requests as done above.

```javascript
import { Client } from 'k6/x/net/http';

export default async function () {
  const client = new Client();
  await client.get('https://httpbin.test.k6.io/get', {
    eventHandlers: {
      'responseReceived': event => {
        const response = event.data;
        // ...
      }
    }
  });
}
```

**TODO**: List other possible event types, and their use cases.


### Implementation

Trying to solve all `new-http` issues with a single large and glorious change wouldn't be reasonable, so improvements will undoubtedly need to be done gradually, in several phases, and over several k6 development cycles.

Note that the implementation process described below is not finalized, and will go through several changes during development.

With this in mind, we propose the following phases:

#### Phase 1: create initial k6 extension

**Goals**:

- Implement a barebones async API that serves as a proof-of-concept for what the final developer experience will look and feel like.

  By barebones, we mean that there must be a `Client` interface with only one method: `request()`, which will work similarly to the current `http.asyncRequest()`. Only `GET` and `POST` methods must be supported.

  The code must be in a state that allows it to be easily extended. Take into account the other design goals of this document, even if they're not ready to be implemented.

- This initial API must solve at least one minor, but concrete, issue of the current API. It should fix something that's currently not possible and doesn't have a good workaround.

  Addressing [#761](https://github.com/grafana/k6/issues/761) would be a good first step.

  As an optional stretch goal, once we settle on the API to configure the transport layer, [#936](https://github.com/grafana/k6/issues/936) and [#970](https://github.com/grafana/k6/issues/970) are good issues to tackle next.


**Non-goals**:

- We won't yet try to solve performance/memory issues of the current API, or implement major new features like data streaming.


#### Phase 2: work on major issues, merge into k6 core as experimental module

**Goals**:

- Work should be started on some of the most impactful issues from the current API.
  Issues like high memory usage when uploading files ([#2311](https://github.com/grafana/k6/issues/2311)), and data streaming ([#592](https://github.com/grafana/k6/issues/592)), are good candidates to focus on first.

- At the end of this phase the API should resolve major limitations of `k6/http`, and it would be a good time to merge it into k6 core as an experimental module (`k6/experimental/net/http`).
  This would make it available to more users, including in the k6 Cloud.


#### Phase 3: work on leftover issues

**Goals**:

- All leftover `new-http` issues should be worked on in this phase.
  **TODO**: Specify which issues and in what order should be worked on here.

- The extension should be thoroughly tested, by both internal and external users.


#### Phase 4: expand, polish and stabilize the API

**Goals**:

- The API should be expanded to include all HTTP methods supported by the current API.
  For the most part, it should reach feature parity with the current API.

- A standalone `fetch()` function should be added that resembles the web Fetch API. There will be some differences in the options compared to the web API, as we want to make parts of the transport/client configurable.

    Internally, this function will create a new client (or reuse a global one?), and will simply act as a convenience wrapper over the underlying `Client`/`Dialer`/`Transport` implementations, which will be initialized with sane default values.

- Towards the end of this phase, the API should be mostly stable, based on community feedback.
  Small changes will be inevitable, but there should be no discussion about the overall design.


#### Phase 5: more testing, deprecating old API

At this point the extension should be relatively featureful and stable to be useful to all k6 users.

**Goals**:

- Continue to gather and address feedback from users, thorough testing and polishing.

- As the final step, we should add deprecation warnings when `k6/http` is used, and point users to the new API. We can also consider promoting the API from experimental to a main module under `k6/net/http`.
  We'll have to maintain both `k6/http` and `k6/net/http` for likely years to come, though any new development will happen in `k6/net/http`, and `k6/http` would only receive bug and security fixes.


## Potential risks

* Long implementation time.

  Not so much of a risk, but more of a necessary side-effect of spreading the work in phases, and over several development cycles. We need this approach in order to have ample time for community feedback, to implement any unplanned features, and to make sure the new API fixes all existing issues.
  Given this, it's likely that the entire process will take many months, possibly more than a year to finalize.


## Technical decisions

TBD after team discussion. In the meantime, see the "Proposed solution(s)" section.
