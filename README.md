Speedboat
=========

Speedboat is the codename for the next generation of Load Impact's load generator.

Getting started
---------------

#### Using Docker

Speedboat is available as a public Docker container for instant execution:

```
docker run loadimpact/speedboat http://example.com
docker run -v $PWD/script.js:/script.js:ro loadimpact/speedboat /script.js
```

#### Compiling from Go source

You can also download and compile the sources:

```
go get github.com/loadimpact/speedboat/cmd/speedboat
```

Requires [a working Go environment](#setting-up-go), version 1.6 or later. Will place the speedboat binary in $GOPATH/bin so you need to have that in your $PATH.

Usage
-----

```
# Run a test against a URL
speedboat http://example.com/

# Run a script
speedboat myscript.js

# Run with 50 VUs for 30 seconds
speedboat -u 50 -d 30s http://example.com/
```


For Go beginners - how to set up Go
-----------------------------------

If you have never worked with Go before, you'll have to follow some steps to get a dev environment going.

This is a tl;dr version of ["How to Write Go Code"](https://golang.org/doc/code.html) from the official documentation - a highly recommended read if you'd like to get more acquainted with these things.

### Quick Installation

1. **Install Go 1.6 or later**
   
   On OSX, you can use [Homebrew](http://brew.sh):
   
   ```
   brew install golang
   ```
   
   On Ubuntu 16.04 or later, you can use the official package:
   
   ```
   sudo apt-get install golang
   ```
   
   On older Ubuntu systems, there's the [Ubuntu Containers Team's PPA](https://launchpad.net/%7Eubuntu-lxc/+archive/ubuntu/lxd-stable):
   
   ```
   sudo add-apt-repository ppa:ubuntu-lxc/lxd-stable
   sudo apt-get update
   sudo apt-get install golang
   ```

2. **Create a Go Workspace**
   
   Just make an empty directory, anywhere you like. This will hold all your Go code. Yes, all of it. See below for an explanation.
   
   ```
   mkdir $HOME/go
   ```
   
   Then export an environment variable called `GOPATH` to point to it, and add `$GOPATH/bin` to your `$PATH`.
   
   ```
   export GOPATH=$HOME/go
   export PATH=$PATH:$GOPATH/bin
   ```
   
   I'd recommend putting this in your `.profile` or similar.

### Understanding `$GOPATH`

Unlike most languages, Go has strong opinions on how your sources should be structured. Rather than having each project in its own, isolated workspace, you have a single workspace for all your Go code.

It looks something like this:

```
src/
   github.com/
       loadimpact/
           speedboat/
               # ...sources...
bin/
   speedboat
```

This is because Go doesn't have a separate package manager, like `pip` or `npm` - instead, package names are repo URLs.

The `go get` command then scans your code for imports, and recursively grabs dependencies using these URLs. In other words, package management is built into the language itself!
