/*
Package fasthttp provides fast HTTP server and client API.

Fasthttp provides the following features:

 1. Optimized for speed. Easily handles more than 100K qps and more than 1M
    concurrent keep-alive connections on modern hardware.

 2. Optimized for low memory usage.

 3. Easy 'Connection: Upgrade' support via RequestCtx.Hijack.

 4. Server provides the following anti-DoS limits:

    - The number of concurrent connections.

    - The number of concurrent connections per client IP.

    - The number of requests per connection.

    - Request read timeout.

    - Response write timeout.

    - Maximum request header size.

    - Maximum request body size.

    - Maximum request execution time.

    - Maximum keep-alive connection lifetime.

    - Early filtering out non-GET requests.

 5. A lot of additional useful info is exposed to request handler:

    - Server and client address.

    - Per-request logger.

    - Unique request id.

    - Request start time.

    - Connection start time.

    - Request sequence number for the current connection.

 6. Client supports automatic retry on idempotent requests' failure.

 7. Fasthttp API is designed with the ability to extend existing client
    and server implementations or to write custom client and server
    implementations from scratch.
*/
package fasthttp
