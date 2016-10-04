Welcome!

This guide will get you from completely clueless about all things ES6, to a veritable expert in the load testing field.

Or at least slightly more clued in than when you started.

Your mileage may vary.

Before we start...
------------------

There's something very important you need to understand about load testing. Web servers, generally speaking, have a limit to how many concurrent connections it can handle. Handling a lot of connections at the same time typically makes responses slower and slower, until at some point the server starts to drop requests to cope with the load.

Speedboat is a tool to measure the performance of your own servers, or others' servers, **with proper permission**. It's **not cool** to run load tests against servers that are not your own without asking first. That's a very good way to make a sysadmin somewhere rather grumpy, and grumpy sysadmins ban people.

tl;dr: **Thou shalt not point load generators at things that are not thine own, lest thy incur the wrath of `iptables -A INPUT -s $IP -j DROP`.**

The anatomy of a script
-----------------------

A test script is a javascript file. But not just any javascript file, an ES6 module that exports (at minimum) a single function:

```es6
export default function() {
    // do something here
}
```

Think of this as your `main()` function in most other languages. Now, you might ask: "why do I need to do this, can't I just write my code outside of this function, like how JS normally works?", which is a perfectly valid question.

The answer lies in how speedboat loads code. Your script is actually run in two phases:

1.  The setup phase. This is run once as the script is being loaded.  
    In this phase, `import` and `export` statements are processed, but it's not run in a "VU context", which means APIs that need a VU (HTTP requests, etc) are not available.

2.  The execution phase. This is when the `default` function is called on each VU iteration.  
    The only thing you may not do in this phase is load other modules that weren't imported during the setup phase.

The reason for this is simply: performance.

If you have 5000 VUs all using the same code, it's obviously much more efficient to parse and compile the script once than to do the exact same work 5000 times.

But it's not just because compilation takes time: 5000 VUs all trying to load script files at the same time would [place a tremendous strain](https://en.wikipedia.org/wiki/Thundering_herd_problem) on both CPU and disk IO throughput, skewing metrics towards the start of the test. By loading all code upfront and completely eliminating disk access at runtime, we can prepare 5000 identical copies of the JS environment, and make VU execution deterministic.

Making HTTP requests
--------------------

The most basic thing you'll probably want to do is make HTTP requests. Fortunately, we have {@link module:speedboat/http|an entire module} dedicated to that.

```es6
import http from "speedboat/http";

export default function() {
    http.get("http://test.loadimpact.com/");
}
```

If you open up the web UI with this test running, you'll see that not only is it sending requests, it's reporting a number of metrics:

* **http_reqs** - counter  
  Total number of HTTP requests.

* **http_req_duration** - min/max/avg/med  
  Total duration of each request, this is the sum of the other metrics + time spent reading the response body.

* **http_req_blocked** - min/max/avg/med  
  Time spent waiting to acquire a socket; this should be close to 0, if it starts rising, it's likely that you're overtaxing your machine.

* **http_req_looking_up** - min/max/avg/med  
  Time spent doing DNS lookups. (DNS records are cached, don't worry.)
  
* **http_req_connecting** - min/max/avg/med  
  Time spent connecting to the remote host. Connections will be reused if possible.
  
* **http_req_sending** - min/max/avg/med  
  Time spent sending a request.
  
* **http_req_waiting** - min/max/avg/med  
  Time between sending the request and the remote host sending a response.
  
* **http_req_receiving** - min/max/avg/med  
  Time spent receiving a response.

While the built-in web dashboard will only display aggregates of all data points, if you output data to InfluxDB or LoadImpact, you'll be able to filter and group the data on various dimensions (eg. URL, response code, etc). See the tutorial: {@tutorial influxdb}.

Tests
-----

Using HTTP requests and varying the number of VUs, you can measure how your servers perform under load. But being able to perform under load isn't much good if your site starts misbehaving - you may be serving errors under higher load, for all you know, and the response times looking fine won't do you much good then.

So let's add some testing to our script.

```es6
import { test } from "speedboat";
import http from "speedboat/http";

export default function() {
    test(http.get("http://test.loadimpact.com/"), {
        "status is 200": (res) => res.status === 200,
    });
}
```

The `test()` function takes a value, and any number of dictionaries of `{ name: fn }`, where `fn` is a function that (optionally) takes a single argument - the value being tested - and returns a truthy value if the test passed. All HTTP requests return a {@link module:speedboat/http.Response}, which among other things contains the response `status` and `body`.

The web UI and will report counters for passes and failures, but note that tests are not assertions - a failed test will not throw an error, and the script will continue regardless.

Groups
------

So far, all we've tested is a single URL. But most sites have a lot more than one page, and APIs typically have more than one endpoint.

You could simply write a bunch of `http.get()` in a sequence... but the test reports would get messy rather quickly - you couldn't tell which tests were for which request. This is when `group()` comes in handy.

```es6
import { test, group } from "speedboat";
import http from "speedboat/http";

// You can reuse commonly used tests like this.
let commonTests = {
    "status is 200": (res) => res.status === 200,
};

export default function() {
    group("front page", function() {
        test(http.get("http://test.loadimpact.com/"), commonTests);
    });
    
    group("pi digits", function() {
        test(http.get("http://test.loadimpact.com/pi.php?decimals=2"), {
            "pi is 3.14": (res) => res.body === "3.14",
        }, commonTests);
    });
}
```

Parsing HTML
------------

So we heard there exist web services that serve HTML. People call them "web sites". Validating the behavior of these "web sites" can be tricky, because they are by nature made to be human-readable, rather than machine-readable.

A naive approach would be to use regular expressions to try to process their content, but... [let's not go there](http://stackoverflow.com/a/1732454/386580). Please. We've been there, and it was naught but misery, bugs and false positives.

So we made the {@link module:speedboat/html|speedboat/html} module for this very task, closely mimicking the good ol' [jQuery](https://jquery.com/) API.

```es6
import { test } from "speedboat";
import http from "speedboat/http";

export default function() {
    let correctTitle = "Welcome to the LoadImpact.com demo site!";
    test(http.get("http://test.loadimpact.com/"), {
        "status is 200": (res) => res.status === 200,
        "greeting is correct": (res) => res.html().find('h2').text() === correctTitle,
    });
}
```
