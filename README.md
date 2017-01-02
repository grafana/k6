k6
=========

k6 is the codename for the next generation of [Load Impact](https://loadimpact.com/)'s load generator.

It features a modern codebase built on [Go](https://golang.org/) and integrates ES6, the latest iteration of Javascript, as a scripting language.

The simplest possible load script would be something along these lines:

```es6
// The script API is provided as ES6 modules, no global namespace pollution.
// If you prefer the older style of doing things, you may also use require().
import http from "k6/http";

// Export your test code as a 'default' function.
export default function() {
	// Make an HTTP request; this will yield a variety of metrics, eg. 'request_duration'.
	http.get("http://test.loadimpact.com/");
}
```

To run it, simply do...

```
$ k6 run script.js
Welcome to k6 v0.4.2!

  execution: local
     output: -
     script: script.js
             ↳ duration: 10s
             ↳ vus: 10, max: 10

  web ui: http://127.0.0.1:6565/

      done [==========================================================]        10s / 10s

    http_req_blocked: avg=19.57µs, max=14.9ms, med=1.28µs, min=808ns, p90=2.27µs, p95=7.1µs
    http_req_connecting: avg=3.25µs, max=7.57ms, med=0s, min=0s, p90=0s, p95=0s
    http_req_duration: avg=5.26ms, max=31.48ms, med=4.3ms, min=2.25ms, p90=7.69ms, p95=12.84ms
    http_req_looking_up: avg=9.12µs, max=7.3ms, med=0s, min=0s, p90=0s, p95=0s
    http_req_receiving: avg=121.95µs, max=13.84ms, med=69.3µs, min=38.57µs, p90=113.79µs, p95=140.04µs
    http_req_sending: avg=18.27µs, max=4.92ms, med=12.09µs, min=6.12µs, p90=22.15µs, p95=28µs
    http_req_waiting: avg=5.1ms, max=30.39ms, med=4.18ms, min=2.17ms, p90=7.33ms, p95=12.22ms
    http_reqs: 17538
    runs: 17538
$
```

Installation
------------

There are a couple of ways to set up k6:

### The simplest way to get started is to use our Docker image

```sh
docker pull loadimpact/k6
docker run --rm --net=host -v myscript.js:/myscript.js loadimpact/k6 run /myscript.js
```

It's recommended to run k6 with `--net=host` as it slightly improves network throughput, and causes container ports to be accessible on the host without explicit exposure. Note that this means opting out of the network isolation normally provided to containers, refer to [the Docker manual](https://docs.docker.com/v1.8/articles/networking/#how-docker-networks-a-container) for more information.


### You can also build k6 from source

This requires a working Go environment (Go 1.7 or later - [set up](https://golang.org/doc/install)) and you will also need node+npm+bower. When you have all prerequisites you can build k6 thus:

```sh
go get -d -u github.com/loadimpact/k6
cd $GOPATH/src/github.com/loadimpact/k6
make
```


#### Step-by-step guide to build k6, starting with a Ubuntu 14.04 Docker image

Following the below steps exactly should result in a working k6 executable:

```sh
docker run -it ubuntu:14.04 /bin/bash
apt-get update
apt-get install git openssh-client make npm curl
ln -s /usr/bin/nodejs /usr/bin/node
npm install -g bower ember-cli@2.7.0
curl https://storage.googleapis.com/golang/go1.7.4.linux-amd64.tar.gz | tar -C /usr/local -xzf -
adduser myuser
```
   
Then you have to create a .gitconfig to make it possible for go to fetch things from a private Github repo:
   
```sh
su - myuser
cat <<EOF >~/.gitconfig
[url "git@github.com:"]
        insteadOf = https://github.com/
EOF
```
   
And then you need to make sure your user has an SSH key that has been authorized access to your Github account. First, create a key:
   
```sh
su - myuser
ssh-keygen
```

Now go to https://github.com/settings/keys and add (the public part of) the new SSH key to your authorized keys.
   
Finally, you're ready to build k6:
   
```sh
su - myuser
export GOROOT=/usr/local/go
export PATH=$PATH:$GOROOT/bin
export GOPATH=$HOME/go
mkdir $GOPATH
go get -d -u github.com/loadimpact/k6
cd $GOPATH/src/github.com/loadimpact/k6
make
```
   
You should now have a k6 binary in your current working directory.
   
   
Usage
-----

k6 works with the concept of "virtual users", or "VUs". A VU is essentially a glorified `while (true)` loop that runs a script over and over and reports stats or errors generated.

Let's say you've written a script called `myscript.js` (you can copy the one from the top of this page), and you want to run it with 100 VUs for 30 seconds. You'd do something like this:

```sh
k6 run -u 100 -d 30s myscript.js
```

The first thing you might notice is that the duration is written "30s", not "30". This is because we're using Go's duration notation, which means `90s`, `1m30s`, `24h` and `2d` are all valid durations, and much more readable than if you had to convert everything to seconds.

The second thing you might notice (or maybe not, if you're just reading this) is that k6 doesn't actually exit immediately after the test finishes. There's a flag to make it (`-q`/`--quit`), but there's a reason for this: it exposes a full-fledged web UI on [http://localhost:6565/](http://localhost:6565/) (by default), which shows realtime statistics and errors.

But that's not the only thing it does. It also exposes a REST API on the same port for controlling test execution, which you can call yourself with an HTTP client of your choice (curl, httpie, ...), or using the commandline wrappers - essentially every k6 command aside from `run` wraps an API call. For example, this will scale the running test down to 50 VUs:

```sh
k6 scale 50
```

This is a quite powerful feature when combined with options like `-d 0` / `--duration 0`, which causes the test to run indefinitely until told otherwise. You're fully in control of how your test is executed!

*For more information, see the included tutorials.*
