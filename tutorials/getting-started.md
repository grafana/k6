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
