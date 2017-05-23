![](logo.png)

**k6** is a modern load testing tool, building on [Load Impact](https://loadimpact.com/)'s years of experience. It provides a clean, approachable scripting API, distributed and cloud execution, and orchestration via a REST API.

This is how load testing should look in the 21st century.

[![](demo.gif)](https://asciinema.org/a/cbohbo6pbkxjwo1k8x0gkl7py)

---

- Project site: [http://k6.io](http://k6.io)

- Documentation: [http://docs.k6.io](http://docs.k6.io)

- Check out k6 on [Slack](https://slackin-defaimlmsd.now.sh/)!

---

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


Running k6
----------

Create a k6 script, to describe what the virtual users should do in your load test:

```javascript
import http from "k6/http";

export default function() {
  http.get("http://test.loadimpact.com");
};
```

Save it as `script.js`, then run k6:

`k6 run script.js`

(Note that if you use the Docker image, the command is slightly different: `docker run -i loadimpact/k6 run - <script.js`)

For more information on how to get started running k6, please look at the [Running k6](https://docs.k6.io/docs/running-k6) documentation.


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
