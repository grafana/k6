# xk6-ssh
A k6 extension for using of SSH in testing. Built for [k6](https://github.com/grafana/k6) using [xk6](https://github.com/grafana/xk6).

## Build

To build a `k6` binary with this extension, first ensure you have the prerequisites:

- [Go toolchain](https://go101.org/article/go-toolchain.html)
- Git

Then:

1. Download `xk6`:
  ```bash
  go install github.com/grafana/xk6/cmd/xk6@latest
  ```

2. Build the binary:
  ```bash
  xk6 build --with github.com/grafana/xk6-ssh@latest
  ```

This will result in a `k6` binary in the current directory.

## Example

### Connect with a password

```javascript
import ssh from 'k6/x/ssh';

export default function () {
  ssh.connect({
    username: `${__ENV.SSH_USER}`,
    password: `${__ENV.SSH_PASSWORD}`,
    host: `${__ENV.SSH_HOST}`,
    port: 22,
  })
  console.log(ssh.run('pwd'))
}
```

Result output:

```plain
$ ./k6 run script.js

         /\      Grafana   /‾‾/  
    /\  /  \     |\  __   /  /   
   /  \/    \    | |/ /  /   ‾‾\ 
  /          \   |   (  |  (‾)  |
 / __________ \  |_|\_\  \_____/ 


     execution: local
        script: script.js
        output: -

     scenarios: (100.00%) 1 scenario, 1 max VUs, 10m30s max duration (incl. graceful stop):
              * default: 1 iterations for each of 1 VUs (maxDuration: 10m0s, gracefulStop: 30s)

time="2026-07-14T20:44:54-04:00" level=info msg="/config\n" source=console


  █ TOTAL RESULTS 

    EXECUTION
    iteration_duration...: avg=17.14ms min=17.14ms med=17.14ms max=17.14ms p(90)=17.14ms p(95)=17.14ms
    iterations...........: 1   58.126017/s

    NETWORK
    data_received........: 0 B 0 B/s
    data_sent............: 0 B 0 B/s




running (00m00.0s), 0/1 VUs, 1 complete and 0 interrupted iterations
default ✓ [ 100% ] 1 VUs  00m00.0s/10m0s  1/1 iters, 1 per VU
```

### Connect with an in-memory private key

When the key is retrieved at runtime (e.g. from a secrets manager such as Vault or
`k6/secrets`), pass its PEM contents via `private_key` instead of a file path via
`rsa_key`. `private_key` takes precedence over `rsa_key` when both are set.

```javascript
import ssh from 'k6/x/ssh';

export default function () {
  ssh.connect({
    username: `${__ENV.SSH_USER}`,
    host: `${__ENV.SSH_HOST}`,
    port: 22,
    private_key: __ENV.SSH_PRIVATE_KEY, // PEM contents, not a path
  })
  console.log(ssh.run('pwd'))
}
```

Result output:

```plain
$ ./k6 run script.js

         /\      Grafana   /‾‾/  
    /\  /  \     |\  __   /  /   
   /  \/    \    | |/ /  /   ‾‾\ 
  /          \   |   (  |  (‾)  |
 / __________ \  |_|\_\  \_____/ 


     execution: local
        script: script.js
        output: -

     scenarios: (100.00%) 1 scenario, 1 max VUs, 10m30s max duration (incl. graceful stop):
              * default: 1 iterations for each of 1 VUs (maxDuration: 10m0s, gracefulStop: 30s)

time="2026-07-14T20:39:53-04:00" level=info msg="/config\n" source=console


  █ TOTAL RESULTS 

    EXECUTION
    iteration_duration...: avg=54.62ms min=54.62ms med=54.62ms max=54.62ms p(90)=54.62ms p(95)=54.62ms
    iterations...........: 1   18.273518/s

    NETWORK
    data_received........: 0 B 0 B/s
    data_sent............: 0 B 0 B/s




running (00m00.1s), 0/1 VUs, 1 complete and 0 interrupted iterations
default ✓ [ 100% ] 1 VUs  00m00.1s/10m0s  1/1 iters, 1 per VU
```

## Testing Locally
This repo includes a [docker-compose.yml](docker-compose.yml) file that starts an [OpenSSH Server](https://docs.linuxserver.io/images/docker-openssh-server) from [LinuxServer.io](https://www.linuxserver.io/).
The `examples` directory contains scripts that are configured to work with this environment out of the box.

> :warning: Be sure that you've already compiled your custom `k6` binary as described in the [Build](#build) section!

We'll use this environment to run some examples.

1. Start the docker compose environment.

   ```shell
   docker compose up -d
   ```
   Once you see the following, you should be ready.
   ```shell
   [+] Running 2/2
    ⠿ Network xk6-ssh_default             Created
    ⠿ Container xk6-ssh-openssh-server-1  Started
   ```
   Next, we'll use the `k6` binary we compiled in the [Build section](#build) above.

1. Using our custom `k6` binary, we can execute our [example scripts](examples/).
   ```shell
   ./k6 run examples/connect-by-rsa-key.js
   ``` 
   The RSA example will then connect to the local SSH server using the `example_rsa` private key.

## FAQ

### How to start `sudo` commands?

Basically we don't provide sudo password autofill. We suggest to use `/etc/sudoers` for this purpose. Please checkout this [article](https://www.cyberciti.biz/faq/linux-unix-running-sudo-command-without-a-password/) for more details.
