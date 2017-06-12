//
// *** v3_account_me.js ***
// This file is an example of how to write test cases for single REST API end points using k6
// It implements a combined functional and load test for the Load Impact REST API end point /account/me
//

import http from "k6/http";
import { group, sleep, check } from "k6";
import { myTrend, options, urlbase, thinktime1, thinktime2 } from "./common.js";
import { v3_account_login } from "./v3_account_login.js";

// Export options object so k6 can access it
export { options };

// Login credentials. We have to be logged on to be able to access the /account/me end point.
// (Note that these credentials do not work, this script is not intended to actually be executed)
let username = "testuser@loadimpact.com";
let password = "testpassword";

// We declare a global variable to hold the API token we need to access the /account/me end point
let api_token = null;

// This function contains the code to actually exercise the /account/me end point
// We export it in case another test wants to use this end point also
export function v3_account_me(token) {
	var url = urlbase + "/v3/account/me";
	return http.get(url, { headers: { "Authorization": "Token " + token } });
};

// This is the "run" function that k6 will call again and again during a load test, or one single
// time when we're running a functional test (1 VU, 1 iteration).
export default function() {
	// The first VU iteration will always perform a login operation in order to get an API 
	// token we need to access the /account/me API end point that we want to test
	if (api_token === null) {
		var res = v3_account_login(username, password);
		var res_json = JSON.parse(res.body);
		api_token = res_json['token']['key'];
	}
	// Below is the actual test case for the /account/me API endpoint
	group("v3_account_me", function() {
		var res = v3_account_me(api_token);
		check(res, {
			"status is 200": (res) => res.status === 200,
			"content-type is application/json": (res) => res.headers['Content-Type'] === "application/json",
			"content OK": (res) => JSON.parse(res.body).hasOwnProperty('organizations')
		});
		myTrend.add(res.timings.duration);
		sleep(thinktime1);
	});
	sleep(thinktime2);
};


