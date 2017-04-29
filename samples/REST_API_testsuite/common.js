//
// *** common.js ***
// This file contains defaults and data that is common to all the individual test cases.
//

import http from "k6/http";
import { Trend } from "k6/metrics";

// Default runtime options
export let options = {
	vus: 1,
	duration: '5s',
	thresholds: {
		transaction_time: ["avg<1000"], // Require transaction_time's average to be <1000ms
		http_req_duration: ["avg<2000"], // Require http_req_duration's average to be <2000ms
	}
};

// Create a Trend metric to hold transaction time data samples from the HTTP calls to the various end points
// Please see note below about this metric and the thresholds set in 'options' above
export let myTrend = new Trend("transaction_time");

// Base URL that we prepend to all URLs we use
export let urlbase = "https://api.staging.loadimpact.com";

// Think times, to slow down execution somewhat
export let thinktime1 = 0.1;
export let thinktime2 = 2.0;



//
//
// About 'transaction_time' and thresholds:
//
// The thresholds defined in the 'options' structure above specifies that the whole k6
// execution should be considered a "fail" if:
//   a) the average value of the metric 'transaction_time' is not less than 1000 ms
//   b) the average value of the metric 'http_req_duration' is not less than 2000 ms
//
// The 'http_req_duration' metric is a standard metric which is always available, but the
// 'transaction_time' metric is something we create specifically.
//
// So, why two metrics? We could have just used the 'http_req_duration' metric and set up one 
// single threshold for that metric. Instead, we define a second Trend metric and add a 
// threshold for that also.
//
// We do this because in all our test cases except one (v3_account_login) each VU will not only
// request the actual API end point we want to test. They will begin by accessing /account/login
// in order to get an API key that is necessary to request the actual end point they want to test.
//
// This means that the 'http_req_duration' metric is going to include response times from those
// calls to /account/login also, which we don't want. In many load test cases it will not matter
// much, as the number of calls to e.g. /account/me in the v3_account_me.js test case will
// greatly outnumber the few calls to /account/login, but to do things "right" we create a
// completely separate metric to hold the response times from our calls to the end points
// we actually want to test.
//
// Note also that the metric we have created is called "transaction_time", but is assigned to
// a variable called "myTrend". It is the myTrend variable that is exported to the different
// test cases and used there, but the threshold setting refer to the "transaction_time" name.
//

