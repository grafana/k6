/**
 * @module k6/http
 */
import { parseHTML } from "k6/html";

const RFC1123_WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const RFC1123_MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

export class CookieJar {
	/**
	 * Represents a HTTP CookieJar.
	 * @memberOf module:k6/http
	 */
	constructor() {
		this.cookies = [];
	}

	static dateToRFC1123(date) {
		// Construct RFC 1123 date string: "ddd, dd mmm yyyy HH:MM:ss Z"
		let weekday = RFC1123_WEEKDAYS[date.getUTCDay()];
		let dayOfMonth = date.getUTCDate();
		if (dayOfMonth < 10) {
			dayOfMonth = `0${dayOfMonth}`;
		}
		let month = RFC1123_MONTHS[date.getUTCMonth()];
		let year = date.getUTCFullYear();
		let hour = date.getUTCHours();
		if (hour < 10) {
			hour = `0${hour}`;
		}
		let minutes = date.getUTCMinutes();
		if (minutes < 10) {
			minutes = `0${minutes}`;
		}
		let seconds = date.getUTCSeconds();
		if (seconds < 10) {
			seconds = `0${seconds}`;
		}
		return `${weekday}, ${dayOfMonth} ${month} ${year} ${hour}:${minutes}:${seconds} GMT`;
	}

	set(key, value, params) {
		if (params === undefined) {
			params = {}
		}
		let expires = params.expires;
		if (expires instanceof Date) {
			expires = CookieJar.dateToRFC1123(expires);
		}
		this.cookies.push({
			"key": key,
			"value": value,
			"domain": params.domain === undefined ? null : params.domain,
			"path": params.path === undefined ? null : params.path,
			"expires": expires === undefined ? null : expires,
			"maxAge": params.maxAge === undefined ? 0 : params.maxAge,
			"secure": params.secure === undefined ? false : params.secure,
			"httpOnly": params.httpOnly === undefined ? false : params.httpOnly
		});
	}

	toString () {
		return "[object CookieJar]";
	}
};

export class Response {
	/**
	 * Represents an HTTP response.
	 * @memberOf module:k6/http
	 */
	constructor(data) {
		Object.assign(this, data);
	}

	json() {
		if (!this._json) {
			this._json = JSON.parse(this.body);
		}
		return this._json;
	}

	html(sel) {
		if (!this._html) {
			this._html = parseHTML(this.body);
		}
		if (sel) {
			return this._html.find(sel);
		}
		return this._html;
	}
}

function parseBody(body) {
	if (body) {
		if (typeof body === "object") {
			let formstring = "";
			for (let key in body) {
				if (formstring !== "") {
					formstring += "&";
				}
				if (Array.isArray(body[key])) {
					let l = body[key].length;
					for (let i = 0; i < l; i++) {
						formstring += key + "=" + encodeURIComponent(body[key][i]);
						if (formstring !== "") {
							formstring += "&";
						}
					}
				} else {
					formstring += key + "=" + encodeURIComponent(body[key]);
				}
			}
			return formstring;
		}
		return body;
	} else {
		return '';
	}
}

/**
 * Makes an HTTP request.
 * @param  {string} method      HTTP Method (eg. "GET")
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body; objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:k6/http.Response}
 */
export function request(method, url, body, params = {}) {
	method = method.toUpperCase();
	if (typeof body === "object") {
		if (typeof params["headers"] !== "object") {
			params["headers"] = {};
		}
		params["headers"]["Content-Type"] = "application/x-www-form-urlencoded";
	}
	body = parseBody(body);
	if (params.hasOwnProperty("cookies")) {
		cookies = [];
		if (typeof params["cookies"] === "object" && params["cookies"] instanceof CookieJar) {
			cookies = params["cookies"].cookies;
		} else if (typeof params["cookies"] === "object") {
			for (let key in params["cookies"]) {
				cookies.push({ "key": key, "value": params["cookies"][key] });
			}
		}
		params["cookies"] = cookies
	}
	return new Response(__jsapi__.HTTPRequest(method, url, body, JSON.stringify(params)));
};

/**
 * Makes a GET request.
 * @see    module:k6/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {Object} params      Additional parameters.
 * @return {module:k6/http.Response}
 */
export function get(url, params) {
	return request("GET", url, null, params);
};

/**
 * Makes a POST request.
 * @see    module:k6/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body; objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:k6/http.Response}
 */
export function post(url, body, params) {
	return request("POST", url, body, params);
};

/**
 * Makes a PUT request.
 * @see    module:k6/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body; objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:k6/http.Response}
 */
export function put(url, body, params) {
	return request("PUT", url, body, params);
};

/**
 * Makes a DELETE request.
 * @see    module:k6/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body; objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:k6/http.Response}
 */
export function del(url, body, params) {
	return request("DELETE", url, body, params);
};

/**
 * Makes a PATCH request.
 * @see    module:k6/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body; objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:k6/http.Response}
 */
export function patch(url, body, params) {
	return request("PATCH", url, body, params);
};

/**
 * Makes a CONNECT request.
 * @see    module:k6/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body; objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:k6/http.Response}
 */
export function connect(url, body, params) {
	return request("CONNECT", url, body, params);
};

/**
 * Makes a OPTIONS request.
 * @see    module:k6/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body; objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:k6/http.Response}
 */
export function options(url, body, params) {
	return request("OPTIONS", url, body, params);
};

/**
 * Makes a TRACE request.
 * @see    module:k6/http.request
 * @param  {string} url         Request URL (eg. "http://example.com/")
 * @param  {string|Object} body Request body; objects will be query encoded.
 * @param  {Object} params      Additional parameters.
 * @return {module:k6/http.Response}
 */
export function trace(url, body, params) {
	return request("TRACE", url, body, params);
};

/**
 * Batches multiple requests together.
 * @see    module:k6/http.request
 * @param  {Array|Object} requests	An array or object of requests, in string or object form.
 * @return {Array.<module:k6/http.Response>|Object}
 */
export function batch(requests) {
	function stringToObject(str) {
		return {
			"method": "GET",
			"url": str,
			"body": null,
			"params": JSON.stringify({})
		}
	}

	function formatObject(obj) {
		obj.params = !obj.params ? {} :obj.params
		obj.body = parseBody(obj.body)
		obj.params = JSON.stringify(obj.params)
		return obj
	}

	let result
	if (requests.length > 0) {
		result = requests.map(e => {
			if (typeof e === 'string') {
				return stringToObject(e)
			} else {
				return formatObject(e)
			}
		})
	} else {
		result = {}
		Object.keys(requests).map(e => {
			let val = requests[e]
			if (typeof val === 'string') {
				result[e] = stringToObject(val)
			} else {
				result[e] = formatObject(val)
			}
		})
	}
	
	let response = __jsapi__.BatchHTTPRequest(result);
	return response
};

export default {
	CookieJar,
	Response,
	request,
	get,
	post,
	put,
	del,
	patch,
	batch,
};
