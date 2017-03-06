![](logo.png)

**k6** is a modern load testing tool, building on [Load Impact](https://loadimpact.com/)'s years of experience. It provides a clean, approachable scripting API, distributed and cloud execution, and orchestration via a REST API.

This is how load testing should look in the 21st century.

[![](demo.gif)](https://asciinema.org/a/cbohbo6pbkxjwo1k8x0gkl7py)

Join our [Slack](https://slackin-defaimlmsd.now.sh/)!

Installation
------------

### Mac

```bash
brew tap loadimpact/k6
brew install k6
```

### Docker

```bash
docker pull loadimpact/k6
```

### Other Platforms

Grab a prebuilt binary from [the Releases page](https://github.com/loadimpact/k6/releases).

Introduction
------------

k6 works with the concept of **virtual users** (VUs), which run scripts - they're essentially glorified, parallel `while(true)` loops. Scripts are written using JavaScript, as ES6 modules, which allows you to break larger tests into smaller pieces, or make reusable pieces as you like.

Scripts must contain, at the very least, a `default` function - this defines the entry point for your VUs, similar to the `main()` function in many other languages:

```bash
export default function() {
    // do things here...
}
```

*"Why not just run my script normally, from top to bottom"*, you might ask - the answer is: we do, but code **inside** and **outside** your `default` function can do different things.

Code inside `default` is called "VU code", and is run over and over for as long as the test is running. Code outside of it is called "init code", and is run only once per VU.

VU code can make HTTP requests, emit metrics, and generally do everything you'd expect a load test to do - with a few important exceptions: you can't load anything from your local filesystem, or import any other modules. This all has to be done from init code.

There are two reasons for this. The first is, of course: performance.

If you read a file from disk on every single script iteration, it'd be needlessly slow; even if you cache the contents of the file and any imported modules, it'd mean the *first run* of the script would be much slower than all the others. Worse yet, if you have a script that imports or loads things based on things that can only be known at runtime, you'd get slow iterations thrown in every time you load something new.

But there's another, more interesting reason. By forcing all imports and file reads into the init context, we design for distributed execution. We know which files will be needed, so we distribute only those files. We know which modules will be imported, so we can bundle them up from the get-go. And, tying into the performance point above, the other nodes don't even need writable filesystems - everything can be kept in-memory.

As an added bonus, you can use this to reuse data between iterations (but only for the same VU):

```js
var counter = 0;

export default function() {
    counter++;
}
```

Development Setup
-----------------

```
go get -u github.com/loadimpact/k6
```

The only catch is, if you want the web UI available, it has to be built separately. Requires a working NodeJS installation.

First, install the `ember-cli` and `bower` tools if you don't have them already:

```
npm install -g ember-cli bower
```

Then build the UI:

```
cd $GOPATH/src/github.com/loadimpact/k6/web
npm install && bower install
ember build
```
