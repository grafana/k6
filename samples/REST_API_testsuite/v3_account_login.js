//
// *** v3_account_login.js ***
// This file is an example of how to write test cases for single REST API end points using k6
// It implements a combined functional and load test for the Load Impact REST API end point /account/login
//

import http from "k6/http";
import { group, sleep, check } from "k6";
import { myTrend, options, urlbase, thinktime1, thinktime2 } from "./common.js";

export { options };

// (Note that these credentials do not work, this script is not intended to actually be executed)
let username = "testuser@loadimpact.com";
let password = "testpassword";

// We export this function as other test cases might want to use it to authenticate
export function v3_account_login(username, password, debug) {
	// First we login. We are not interested in performance metrics from these login transactions
	var url = urlbase + "/v3/account/login";
	var payload = { email: username, password: password };
	var res = http.post(url, JSON.stringify(payload), { headers: { "Content-Type": "application/json" } });
	if (typeof debug !== 'undefined')
		console.log("Login: status=" + String(res.status) + "  Body=" + res.body);
	return res;
};

// Exercise /login endpoint when this test case is executed
export default function() {
	group("login", function() {
		var res = v3_account_login(username, password);
		check(res, {
			"status is 200": (res) => res.status === 200,
			"content-type is application/json": (res) => res.headers['Content-Type'] === "application/json",
			"login successful": (res) => JSON.parse(res.body).hasOwnProperty('token')
		});
		myTrend.add(res.timings.duration);
		sleep(thinktime1);
	});
};
