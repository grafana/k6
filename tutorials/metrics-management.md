#k6 Metrics Management#

This section covers the important aspect of metrics management in k6 - built-in implicit structures and custom explicit features.

##Per-request metrics##
```es6
var res = http.get(“http://httpbin.org” (http://httpbin.xn--org-9o0a/));
console.log(“Response time was “ + String(res.timings.duration) + “ ms”);
```

In the above snippet, `res` is an HTTP response object containing:

* `res.body` (`string` containing the HTTP response body)
* `res.headers` (`dictionary` containing header-name/header-value pairs)
* `res.status` (`integer` contaning HTTP response code received from server)
* `res.timings` (`dictionary` containing HTTP timing information for the request)
    * `res.timings.blocked` (`float` containing time (ms) spent blocked before initiating request)
    * `res.timings.looking_up` (`float` containing time (ms) spent looking up host name in DNS)
    * `res.timings.connecting` (`float` containing time (ms) spent setting up TCP connection to host)
    * `res.timings.sending` (`float` containing time (ms) spent sending request)
    * `res.timings.waiting` (`float` containing time (ms) spent waiting for server response (a.k.a. TTFB))
    * `res.timings.receiving` (`float` containing time (ms) spent receiving response data)
    * `res.timings.duration` (`float` containing total time (ms) for request, excluding *blocked* time)

##Custom metrics##
Custom metrics are reported at the end of a load test, and can also be used with thresholds to provide pass/fail functionality for a load test, like this:
```es6
export let options = {                                                     
   thresholds: {                                                           
      request_duration: ["avg<100"],                                       
   }                                                                       
};                                                                         
                                                                           
var myTrend = new Trend(“request_duration”);                               
                                                                           
export default function() {                                                
   var r = http.get("https://httpbin.org");                                
   myTrend.add(r.timings.duration);                                        
};                                                                         
```
The above code will:

* create a Trend metric named “request_duration” and referred to in the code using the variable name myTrend
* define a threshold for the Trend metric. This threshold says that the load test should fail if the average value of the Trend metric goes below 100. This means that if at any time during the load test, the currently computed average of all sample values added to myTrend is less than 100, then the whole load test will be marked as failed.
* create a default function that will be executed repeatedly by all VUs in the load test. This function makes an HTTP request and adds the HTTP duration (request time - response.timings.duration) to the Trend metric.

For a combined functional and load test, the code can look like this:
```es6
export let options = {                                                     
   thresholds: {                                                           
      request_duration: ["avg<100"],                                       
   }                                                                       
};                                                                         
                                                                           
var myTrend = new Trend(“request_duration”);                               
                                                                           
export default function() {                                                
   var r = http.get("https://httpbin.org");                                
   myTrend.add(r.timings.duration);                                        
   check(r, {                                                              
      "status is 200": (r) => r.status === 200,                            
      "body size 1234 bytes": (r) => r.body.length === 1234                
   });                                                                     
};                                                                         
```
The above code can be run both as a load test or as a functional test. Some things worth thinking about when running a multi-functional/combined test case:

* **Preventing functest check()s from causing load test false positives**
    In the load test use case you *may* want to “tune down” the sensitivity of the check()s so a single check failure will not fail the whole load test. This can be done using the —acceptance (-a) command-line option that allows you to specify what percentage of check()s may fail without the whole load test being marked as a fail.
* **Preventing load test thresholds from causing functest false positives**
    *Can we do this, currently?  I.e. in the example above the “average” response time for a single transaction may be high due to DNS lookup times or whatnot, causing the functest to always fail because of its load test threshold.*

###Available custom metrics###

####Counter *(cumulative metric)*####
```es6
var myCounter = new Counter(“my_counter”);
myCounter.add(1);
myCounter.add(2);
```    
The above code will make k6 print `my_counter: 3` at the end of the test. There is currently no way of accessing the value of any custom metric from within Javascript. Note also that counters that have value zero (`0`) at the end of a test are a special case - they will **NOT** be printed to the stdout summary.
    
####Gauge *(keep the latest value only)*####
```es6
var myGauge = new Gauge(“my_gauge”);
myGauge.add(1);
myGauge.add(2);
```    
The above code will make k6 print `my_gauge: 2` at the end of the test. As with the Counter metric above, a Gauge with value zero (`0`) will **NOT** be printed to the stdout summary at the end of the test.
    
####Trend *(collect trend statistics (min/max/avg/med) for a series of values)*####
```es6
var myTrend = new Trend(“my_trend”);
myTrend.add(1);
myTrend.add(2);
```    
The above code will make k6 print `my_trend: min=1, max=2, avg=1.5, med=1.5`
    
####Rate *(keeps track of percentage of values in a series that are non-zero)*####
```es6    
var myRate = new Rate(“my_rate”);
myRate.add(true);
myRate.add(false);
myRate.add(1);
myRate.add(0);
```    
The above code will make k6 print `my_rate: 50.00%` at the end of the test.
    
    
###Notes###
* custom metrics are only collected from VU threads at the end of a VU iteration, which means that for long-running scripts you may not see any custom metrics until a while into the test.
    
* thresholds are only checked every 2 seconds, which both means you incur up to a 2-second delay before a load test is failed once failure criteria has been reached. It also means that there is a built-in 2-second grace period at the start of a load test where e.g. the above `rate>0.75` threshold would not cause the test to fail, despite the initial value for the metric being `0` (zero).

##Assertions for functional testing##
Checks are used for functional testing:
```es6
var res = http.get(“http://example.com” (http://example.xn--com-9o0a/));
check(res, { “status is 200”: (res) => res.status === 200 });  // returns false if check condition fails
```
Multiple checks can be included in a single check() call:
```es6
var res = http.get(“http://example.com” (http://example.xn--com-9o0a/));
check(res, { 
   “status is 200”: (res) => res.status === 200 },
   "content size = 1234 bytes": (res) => res.body.length === 1234)
});
```
The percentage of successful checks are printed to stdout after the end of the load test. By default, a failed `check()` will result in the whole load test being marked as failed.

##Load test failure thresholds##
There are three “normal” ways a load test can fail currently:

1. Upon failure of a `check()` the whole load test is marked as failed by default. This behaviour will be possible to override, see section above.

1. `taint()` allows a VU to mark a whole load test as failed:
  ```es6    
if (something_happened) {
    taint(); // marks test as failed, but does not exit
}
  ```

1. Upon an `options.thresholds` condition failing:
  ```es6    
export let options = {                                      
    thresholds: {                                            
        my_duration: ["avg<100"]                              
    }                                                        
};
  ```
  Thresholds operate on any metrics (in this case a custom metric called `my_duration`) and allow you to specify one or more strings containing JS code that will be “evaluated” to verify if the threshold is OK (is a "pass") or not.
