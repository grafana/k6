Speedboat
=========

Speedboat is the codename for the next generation of [Load Impact](https://loadimpact.com/)'s load generator.

It features a modern codebase built on [Go](https://golang.org/) and integrates ES6, the latest iteration of Javascript, as a scripting language.

The simplest possible load script would be something along these lines:

```es6
// The script API is provided as ES6 modules, no global namespace pollution.
// If you prefer the older style of doing things, you may also use require().
import http from "speedboat/http";

// Export your test code as a 'default' function.
export default function() {
	// Make an HTTP request; this will yield a variety of metrics, eg. 'request_duration'.
	http.get("http://test.loadimpact.com/");
}
```

To run it, simply do...

```
speedboat run script.js
```

Installation
------------

There are a couple of ways to set up Speedboat:

1. The **recommended way** to get started is to grab a binary release from [the releases page](https://github.com/loadimpact/speedboat/releases). Either copy/link the `speedboat` binary somewhere in your `$PATH`, or use it as:

   ```sh
   ./speedboat run myscript.js
   ```

1. If you're comfortable using Docker, you may use that as well:

   ```sh
   docker pull loadimpact/speedboat
   docker run --rm --net=host -v myscript.js:/myscript.js loadimpact/speedboat run /myscript.js
   ```

   It's recommended to run speedboat with `--net=host` as it slightly improves network throughput, and causes container ports to be accessible on the host without explicit exposure. Note that this means opting out of the network isolation normally provided to containers, refer to [the Docker manual](https://docs.docker.com/v1.8/articles/networking/#how-docker-networks-a-container) for more information.

1. If you have a Go environment [set up](https://golang.org/doc/install), you may simply use `go get`:

   ```sh
   go get github.com/loadimpact/speedboat
   ```

   Use `go get -u` to pull down updates.

Usage
-----

Speedboat works with the concept of "virtual users", or "VUs". A VU is essentially a glorified `while (true)` loop that runs a script over and over and reports stats or errors generated.

Let's say you've written a script called `myscript.js` (you can copy the one from the top of this page), and you want to run it with 100 VUs for 30 seconds. You'd do something like this:

```sh
speedboat run -u 100 -d 30s myscript.js
```

The first thing you might notice is that the duration is written "30s", not "30". This is because we're using Go's duration notation, which means `90s`, `1m30s`, `24h` and `2d` are all valid durations, and much more readable than if you had to convert everything to seconds.

The second thing you might notice (or maybe not, if you're just reading this) is that Speedboat doesn't actually exit immediately after the test finishes. There's a flag to make it (`-q`/`--quit`), but there's a reason for this: it exposes a full-fledged web UI on [http://localhost:6565/](http://localhost:6565/) (by default), which shows realtime statistics and errors.

But that's not the only thing it does. It also exposes a REST API on the same port for controlling test execution, which you can call yourself with an HTTP client of your choice (curl, httpie, ...), or using the commandline wrappers - essentially every speedboat command aside from `run` wraps an API call. For example, this will scale the running test down to 50 VUs:

```sh
speedboat scale 50
```

This is a quite powerful feature when combined with options like `-d 0` / `--duration 0`, which causes the test to run indefinitely until told otherwise. You're fully in control of how your test is executed!

*For more information, see the included tutorials.*
