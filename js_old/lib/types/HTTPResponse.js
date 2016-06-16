"use strict";

var HTTPResponse = {};

HTTPResponse.json = function() {
	return JSON.parse(this.body);
}
