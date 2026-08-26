# xk6-redis

This is an official [k6](https://github.com/grafana/k6) extension, it's a client library for [Redis](https://redis.io) database. Check out the extension's documentation [here](https://grafana.com/docs/k6/latest/javascript-api/k6-experimental/redis).

## Get started

Add the extension's import
```js
import redis from "k6/x/redis";
```

Define the client
```js
import redis from "k6/x/redis";

// Instantiate a new Redis client using a URL.
// The connection will be established on the first command call.
const client = new redis.Client('redis://localhost:6379');
```

Call the API from the VU context

```js
export default function () {
  redisClient.sadd("crocodile_ids", ...crocodileIDs);
...
}
```

## Build

The most common and simple case is to use k6 with automatic extension resolution. Simply add the extension's import and k6 will resolve the dependency automtically.  
However, if you prefer to build it from source using xk6, first ensure you have the prerequisites:

- [Go toolchain](https://go101.org/article/go-toolchain.html)
- Git

Then:

1. Install `xk6`:
  ```shell
  go install go.k6.io/xk6/cmd/xk6@latest
  ```

2. Build the binary:
  ```shell
  xk6 build --with github.com/grafana/xk6-redis
  ```
