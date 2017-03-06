#k6 Metrics Management#

This section covers the important aspect of metrics management in k6 - built-in implicit structures and custom explicit features.

##Per-request metrics##
```es6
import http from "k6/http";

export default function() {
  var res = http.get(“http://httpbin.org”);
  console.log(“Response time was “ + String(res.timings.duration) + “ ms”);
};
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
import http from "k6/http";
import { Trend } from "k6/metrics";

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
* define a threshold for the Trend metric. This threshold says that the load test should fail if the average value of the Trend metric was lower than 100 at the end of the test.
* create a default function that will be executed repeatedly by all VUs in the load test. This function makes an HTTP request and adds the HTTP duration (request time - response.timings.duration) to the Trend metric.

For a combined functional and load test, the code can look like this:
```es6
import http from "k6/http";
import { Trend } from "k6/metrics";
import { check, taint } from "k6";

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
   }) || taint();                                                                     
};                                                                         
```
The above code can be run both as a load test or as a functional test. If any of the check() conditions fail, the taint() function will be called, which marks the whole test run as a "fail". This means k6 will exit with a non-zero exit code, informing e.g. a CI system that the whole test was a fail. (Normally, results from check()s are stored as metrics but do not affect the pass/fail status of the whole test run)


###Available custom metrics###

####Counter *(cumulative metric)*####
```es6
import { Counter } from "k6/metrics";

var myCounter = new Counter(“my_counter”);
myCounter.add(1);
myCounter.add(2);
```    
The above code will make k6 print `my_counter: 3` at the end of the test. There is currently no way of accessing the value of any custom metric from within Javascript. Note also that counters that have value zero (`0`) at the end of a test are a special case - they will **NOT** be printed to the stdout summary.
    
####Gauge *(keep the latest value only)*####
```es6
import { Gauge } from "k6/metrics";

var myGauge = new Gauge(“my_gauge”);
myGauge.add(1);
myGauge.add(2);
```    
The above code will make k6 print `my_gauge: 2` at the end of the test. As with the Counter metric above, a Gauge with value zero (`0`) will **NOT** be printed to the stdout summary at the end of the test.
    
####Trend *(collect trend statistics (min/max/avg/med) for a series of values)*####
```es6
import { Trend } from "k6/metrics";

var myTrend = new Trend(“my_trend”);
myTrend.add(1);
myTrend.add(2);
```    
The above code will make k6 print `my_trend: min=1, max=2, avg=1.5, med=1.5`
    
####Rate *(keeps track of percentage of values in a series that are non-zero)*####
```es6
import { Rate } from "k6/metrics";

var myRate = new Rate(“my_rate”);
myRate.add(true);
myRate.add(false);
myRate.add(1);
myRate.add(0);
```    
The above code will make k6 print `my_rate: 50.00%` at the end of the test.
    
    
###Notes###
* custom metrics are only collected from VU threads at the end of a VU iteration, which means that for long-running scripts you may not see any custom metrics until a while into the test.


##Assertions for functional testing##
Checks are used for functional testing:
```es6
var res = http.get(“http://example.com”);
check(res, { “status is 200”: (res) => res.status === 200 });  // returns false if check condition fails
```
Multiple checks can be included in a single check() call:
```es6
var res = http.get(“http://example.com”);
check(res, { 
   “status is 200”: (res) => res.status === 200 },
   "content size = 1234 bytes": (res) => res.body.length === 1234)
});
```
The percentage of successful checks are printed to stdout after the end of the load test.

##Load test failure thresholds##
There are two “normal” ways a load test can fail currently:

1. `taint()` allows a VU to mark a whole load test as failed:
  ```es6    
if (something_happened) {
    taint(); // marks test as failed, but does not exit
}
  ```

2. Upon an `options.thresholds` condition failing:
  ```es6    
export let options = {                                      
    thresholds: {                                            
        my_duration: ["avg<100"]                              
    }                                                        
};
  ```
  Thresholds operate on any metrics (in this case a custom metric called `my_duration`) and allow you to specify one or more strings containing JS code that will be “evaluated” to verify if the threshold is OK (is a "pass") or not.
