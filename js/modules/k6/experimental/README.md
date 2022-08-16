# Experimental Modules

This folder are here as a documentation and reference point for k6's experimental modules. 

Although [accessible in k6 scripts](../../../initcontext.go) under the `k6/experimental` import path, those modules implementations live in their own repository and are not part of the k6 stable release yet:
* [`k6/experimental/k6-redis`](https://github.com/grafana/xk6-redis)
* [`k6/experimental/k6-websockets`](https://github.com/grafana/xk6-websockets)
* [`k6/experimental/k6-timers`](https://github.com/grafana/xk6-timers)

While we intend to keep these modules as stable as possible, we may need to add features or introduce breaking changes. This could happen at any time until we release the module as stable. **use them at your own risk**.
