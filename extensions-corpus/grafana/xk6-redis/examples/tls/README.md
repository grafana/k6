# How to run a k6 test against a Redis test server with TLS

1. Move in the docker folder `cd docker`
2. Run `sh gen-test-certs.sh` to generate custom TLS certificates that the docker container will use.
3. Run `docker-compose up` to start the Redis server with TLS enabled.
4. Connect to it with `redis-cli --tls --cert ./tests/tls/redis.crt --key ./tests/tls/redis.key --cacert ./tests/tls/ca.crt` and run `AUTH tjkbZ8jrwz3pGiku` to authenticate, and verify that the redis server is properly set up.
5. Build the k6 binary with `xk6 build --with github.com/k6io/xk6-redis=.`
5. Run `./k6 run loadtest-tls.js` to run the k6 load test with TLS enabled.