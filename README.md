Speedboat
=========

Speedboat is the codename for the next generation of Load Impact's load generator.

Installation
------------

### On your own machine

```
go get github.com/loadimpact/speedboat
```

Make sure you have Go 1.6 or later installed. It will be installed to `$GOPATH/bin`, so you probably want that in your PATH.

### Using Docker

```
docker pull loadimpact/speedboat
```

You can now run speedboat using `docker run loadimpact/speedboat [...]`.

Substitute the `speedboat` command for this in the instructions below if using this method.

Running (standalone)
--------------------

```
speedboat run --script scripts/google.js
```

This will run a very simple load test against `https://google.com/` for 10 seconds (change with eg `-d 15s` or `-d 2m`), using 2 VUs (change with eg `-u 4`).
